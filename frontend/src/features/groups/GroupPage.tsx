import {
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogContent,
  DialogTitle,
  IconButton,
  LinearProgress,
  List,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Snackbar,
  Stack,
  Tab,
  Tabs,
  Typography
} from "@mui/material";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useRef, useState, type PointerEvent } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { api, refreshTaskStatus } from "../../api/client";
import { qrImageURL } from "../../api/qr";
import type { GroupResponse, MeResponse, MemberCompletion, TasksResponse, TaskStatus, User } from "../../api/types";
import {
  applyCompletionToTasks,
  flushCompletionQueue,
  pendingCompletionCount,
  queueCompletion,
  trySyncCompletion,
  type PendingCompletion
} from "../../offline/completionSync";
import { loadGroupSnapshot, saveGroupSnapshot } from "../../offline/groupSnapshot";
import {
  ArchiveIcon,
  ArrowBackIcon,
  CheckIcon,
  CloseIcon,
  ContentCopyIcon,
  EditIcon,
  ExpandMoreIcon,
  InteractionIcon,
  LockIcon,
  LockOpenIcon,
  MoreVertIcon,
  NavigateBeforeIcon,
  NavigateNextIcon,
  PersonRemoveIcon,
  StageIcon,
  SyncIcon,
  VenueIcon
} from "../../icons";
import { EditGroupDialog } from "./GroupDialogs";

export function GroupPage() {
  const { groupId = "" } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [uiVisible, setUiVisible] = useState(true);
  const [taskIndex, setTaskIndex] = useState(0);
  const [memberIndex, setMemberIndex] = useState(0);
  const [taskPickerOpen, setTaskPickerOpen] = useState(false);
  const [selectedTaskGroup, setSelectedTaskGroup] = useState("");
  const [offlineSnapshot, setOfflineSnapshot] = useState(false);
  const [manageAnchor, setManageAnchor] = useState<HTMLElement | null>(null);
  const [editOpen, setEditOpen] = useState(false);
  const [copyMessage, setCopyMessage] = useState("");
  const [pendingCount, setPendingCount] = useState(() => pendingCompletionCount());
  const [isOnline, setIsOnline] = useState(() => navigator.onLine);
  const pointerStartX = useRef<number | null>(null);
  const suppressNextToggle = useRef(false);
  const me = queryClient.getQueryData<MeResponse>(["me"]);

  const group = useQuery({
    queryKey: ["group", groupId],
    queryFn: async () => {
      try {
        const response = await api<GroupResponse>(`/group/detail?groupId=${encodeURIComponent(groupId)}`);
        setOfflineSnapshot(false);
        return response;
      } catch (error) {
        const snapshot = loadGroupSnapshot(groupId);
        if (snapshot) {
          setOfflineSnapshot(true);
          return { group: snapshot.group };
        }
        throw error;
      }
    },
    enabled: Boolean(groupId)
  });
  const tasksQuery = useQuery({
    queryKey: ["groupTasks", groupId],
    queryFn: async () => {
      try {
        const response = await api<TasksResponse>(`/group/tasks?groupId=${encodeURIComponent(groupId)}`);
        setOfflineSnapshot(false);
        return response;
      } catch (error) {
        const snapshot = loadGroupSnapshot(groupId);
        if (snapshot) {
          setOfflineSnapshot(true);
          return { tasks: snapshot.tasks };
        }
        throw error;
      }
    },
    enabled: Boolean(groupId)
  });

  const tasks = tasksQuery.data?.tasks ?? [];
  const currentTask = tasks[Math.min(taskIndex, Math.max(tasks.length - 1, 0))];
  const groupedTasks = useMemo(() => groupTasksForPicker(tasks), [tasks]);
  const selectedTaskGroupName = selectedTaskGroup || currentTask?.groupName || groupedTasks[0]?.name || "";
  const visibleTaskGroup = groupedTasks.find((item) => item.name === selectedTaskGroupName) ?? groupedTasks[0];
  const members = currentTask?.members ?? [];
  const currentMember = members[Math.min(memberIndex, Math.max(members.length - 1, 0))];
  const currentMemberQR = qrImageURL(currentMember?.member);
  const currentGroup = group.data?.group;
  const isOwner = currentGroup?.role === "owner";
  const isArchived = Boolean(currentGroup?.archivedAt);

  const complete = useMutation({
    mutationFn: async (action: Omit<PendingCompletion, "id">) => {
      try {
        await trySyncCompletion(action);
        return { action, synced: true };
      } catch (error) {
        if (navigator.onLine) throw error;
        queueCompletion(action);
        setPendingCount(pendingCompletionCount());
        return { action, synced: false };
      }
    },
    onMutate: async (action) => {
      await queryClient.cancelQueries({ queryKey: ["groupTasks", groupId] });
      const previous = queryClient.getQueryData<TasksResponse>(["groupTasks", groupId]);
      const checkedBy = currentUser(me?.user);
      const nextTasks = applyCompletionToTasks(previous?.tasks ?? tasks, action, checkedBy);
      queryClient.setQueryData<TasksResponse>(["groupTasks", groupId], { tasks: nextTasks });
      if (group.data?.group) {
        await saveGroupSnapshot({ group: group.data.group, tasks: nextTasks, savedAt: new Date().toISOString() });
      }
      return { previous };
    },
    onError: (_error, _action, context) => {
      if (context?.previous) queryClient.setQueryData(["groupTasks", groupId], context.previous);
    },
    onSuccess: ({ action, synced }) => {
      if (synced) void queryClient.invalidateQueries({ queryKey: ["groupTasks", groupId] });
      if (action.completed) advanceToNextIncomplete(action.userId);
    }
  });

  const refreshStatus = useMutation({
    mutationFn: (entry: MemberCompletion) => {
      if (!currentTask) throw new Error("当前点位不存在");
      return refreshTaskStatus({ groupId, taskId: currentTask.id, userId: entry.member.id });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["groupTasks", groupId] });
    }
  });

  const removeMember = useMutation({
    mutationFn: async (userId: string) => {
      await api<{ ok: boolean }>("/group/member/remove", {
        method: "POST",
        body: JSON.stringify({ groupId, userId })
      });
    },
    onSuccess: async () => {
      setMemberIndex(0);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["groupTasks", groupId] }),
        queryClient.invalidateQueries({ queryKey: ["group", groupId] }),
        queryClient.invalidateQueries({ queryKey: ["groups"] })
      ]);
    }
  });

  const setJoinLocked = useMutation({
    mutationFn: async (locked: boolean) => {
      await api<{ group: typeof currentGroup }>(locked ? "/group/join-lock" : "/group/join-unlock", {
        method: "POST",
        body: JSON.stringify({ groupId })
      });
    },
    onSuccess: async () => {
      setManageAnchor(null);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["group", groupId] }),
        queryClient.invalidateQueries({ queryKey: ["groups"] })
      ]);
    }
  });

  const archiveGroup = useMutation({
    mutationFn: async () => {
      await api<{ group: typeof currentGroup }>("/group/archive", {
        method: "POST",
        body: JSON.stringify({ groupId })
      });
    },
    onSuccess: async () => {
      setManageAnchor(null);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["group", groupId] }),
        queryClient.invalidateQueries({ queryKey: ["groups"] }),
        queryClient.invalidateQueries({ queryKey: ["groupTasks", groupId] })
      ]);
    }
  });

  const syncTasks = useMutation({
    mutationFn: async () => {
      await api("/group/task/sync", {
        method: "POST",
        body: JSON.stringify({ groupId })
      });
    },
    onSuccess: async () => {
      setManageAnchor(null);
      setCopyMessage("乐园任务已同步");
      await tasksQuery.refetch();
    },
    onError: (error) => {
      setManageAnchor(null);
      setCopyMessage(error instanceof Error ? error.message : "乐园任务同步失败");
    }
  });

  const loading = group.isLoading || tasksQuery.isLoading;

  function shiftMember(delta: number) {
    if (members.length === 0) return;
    setMemberIndex((index) => (index + delta + members.length) % members.length);
  }

  function selectTask(task: TaskStatus) {
    const nextIndex = tasks.findIndex((item) => item.id === task.id);
    setTaskIndex(Math.max(nextIndex, 0));
    setMemberIndex(0);
    setTaskPickerOpen(false);
  }

  function toggleCompletion(entry: MemberCompletion) {
    if (!currentTask || isArchived || !canToggleCompletion(entry)) return;
    complete.mutate({
      groupId,
      taskId: currentTask.id,
      userId: entry.member.id,
      completed: !entry.completed,
      updatedAt: new Date().toISOString()
    });
  }

  function advanceToNextIncomplete(completedUserId: string) {
    const latest = queryClient.getQueryData<TasksResponse>(["groupTasks", groupId])?.tasks ?? tasks;
    const task = latest.find((item) => item.id === currentTask?.id);
    const latestMembers = task?.members ?? [];
    if (latestMembers.length === 0) return;
    const current = latestMembers.findIndex((entry) => entry.member.id === completedUserId);
    for (let offset = 1; offset <= latestMembers.length; offset += 1) {
      const nextIndex = (Math.max(current, 0) + offset) % latestMembers.length;
      if (!latestMembers[nextIndex]?.completed) {
        setMemberIndex(nextIndex);
        return;
      }
    }
  }

  async function copyInviteLink() {
    const invite = `${window.location.origin}/?invite=${encodeURIComponent(groupId)}`;
    await navigator.clipboard.writeText(invite);
    setCopyMessage("已复制邀请链接");
    setManageAnchor(null);
  }

  function canRemove(entry: MemberCompletion) {
    return isOwner && !isArchived && entry.member.id !== me?.user.id;
  }

  function confirmRemove(entry: MemberCompletion) {
    if (window.confirm(`确认移除 ${entry.member.displayName}？`)) {
      removeMember.mutate(entry.member.id);
    }
  }

  function confirmArchive() {
    if (window.confirm("确认归档这个互助组？归档后不能继续打卡。")) {
      archiveGroup.mutate();
    }
  }

  function handlePointerDown(event: PointerEvent<HTMLElement>) {
    if (event.pointerType === "mouse" && event.button !== 0) return;
    pointerStartX.current = event.clientX;
  }

  function handlePointerUp(event: PointerEvent<HTMLElement>) {
    if (pointerStartX.current === null) return;
    const delta = event.clientX - pointerStartX.current;
    pointerStartX.current = null;
    if (Math.abs(delta) < 48) return;
    suppressNextToggle.current = true;
    shiftMember(delta > 0 ? -1 : 1);
  }

  useEffect(() => {
    const groupValue = group.data?.group;
    if (!groupValue || !tasksQuery.data?.tasks) return;
    void saveGroupSnapshot({ group: groupValue, tasks: tasksQuery.data.tasks, savedAt: new Date().toISOString() });
  }, [group.data?.group, tasksQuery.data?.tasks]);

  useEffect(() => {
    if (!taskPickerOpen) return;
    setSelectedTaskGroup(currentTask?.groupName || groupedTasks[0]?.name || "");
  }, [currentTask?.groupName, groupedTasks, taskPickerOpen]);

  useEffect(() => {
    async function syncPending() {
      const remaining = await flushCompletionQueue();
      setPendingCount(remaining);
      if (remaining === 0) {
        await queryClient.invalidateQueries({ queryKey: ["groupTasks", groupId] });
      }
    }
    if (navigator.onLine) void syncPending();
    function handleOnline() {
      setIsOnline(true);
      void syncPending();
    }
    function handleOffline() {
      setIsOnline(false);
    }
    window.addEventListener("online", handleOnline);
    window.addEventListener("offline", handleOffline);
    return () => {
      window.removeEventListener("online", handleOnline);
      window.removeEventListener("offline", handleOffline);
    };
  }, [groupId, queryClient]);

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (taskPickerOpen || manageAnchor) return;
      if (event.key === "ArrowLeft") {
        event.preventDefault();
        shiftMember(-1);
      }
      if (event.key === "ArrowRight") {
        event.preventDefault();
        shiftMember(1);
      }
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [manageAnchor, members.length, taskPickerOpen]);

  const completedLabel = useMemo(() => {
    if (!currentMember?.completedAt) return "";
    const date = new Date(currentMember.completedAt);
    const time = Number.isNaN(date.getTime())
      ? currentMember.completedAt
      : date.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
    return `${time} ${currentMember.checkedByName}`;
  }, [currentMember]);

  const currentMemberLive = currentMember ? isLiveCompletion(currentMember) : false;
  const currentMemberCanRefresh = currentMember ? canRefreshCompletion(currentMember) : false;

  if (loading) {
    return (
      <Stack sx={{ minHeight: "100vh", alignItems: "center", justifyContent: "center" }}>
        <CircularProgress />
      </Stack>
    );
  }

  return (
    <Box
      className={`qr-viewer ${uiVisible ? "ui-visible" : "ui-hidden"}`}
      onPointerDown={handlePointerDown}
      onPointerUp={handlePointerUp}
    >
      <Box
        className="qr-photo-stage"
        onClick={() => {
          if (suppressNextToggle.current) {
            suppressNextToggle.current = false;
            return;
          }
          setUiVisible((visible) => !visible);
        }}
      >
        {currentMemberQR ? (
          <img className="qr-photo-img" src={currentMemberQR} alt={`${currentMember.member.displayName} 的二维码`} />
        ) : (
          <Box className="qr-missing">
            <Typography variant="h5" sx={{ fontWeight: 800 }}>
              未上传二维码
            </Typography>
            <Typography color="text.secondary">当前成员还没有二维码图片。</Typography>
          </Box>
        )}
      </Box>

      <Box className="qr-overlay qr-overlay-top" onClick={(event) => event.stopPropagation()}>
        <Stack direction="row" sx={{ alignItems: "center", justifyContent: "space-between", gap: 2 }}>
          <Stack direction="row" sx={{ alignItems: "center", gap: 1.5 }}>
            <IconButton color="inherit" onClick={() => navigate("/")}>
              <ArrowBackIcon />
            </IconButton>
            <Box>
              <Typography variant="h5" sx={{ fontWeight: 850 }}>
                {currentGroup?.name}
              </Typography>
            </Box>
          </Stack>
          <Stack direction="row" sx={{ alignItems: "center", gap: 1 }}>
            {(!isOnline || offlineSnapshot) && <Chip label="离线模式" color="warning" size="small" />}
            {isArchived && <Chip label="已归档" color="default" size="small" />}
            {isOwner && (
              <IconButton aria-label="互助组管理" color="inherit" onClick={(event) => setManageAnchor(event.currentTarget)}>
                <MoreVertIcon />
              </IconButton>
            )}
          </Stack>
        </Stack>
      </Box>

      <IconButton className="qr-nav qr-nav-left" onClick={(event) => { event.stopPropagation(); shiftMember(-1); }}>
        <NavigateBeforeIcon />
      </IconButton>
      <IconButton className="qr-nav qr-nav-right" onClick={(event) => { event.stopPropagation(); shiftMember(1); }}>
        <NavigateNextIcon />
      </IconButton>

      <Box className="qr-overlay qr-overlay-bottom" onClick={(event) => event.stopPropagation()}>
        <Box className="task-sheet">
          <Box
            component="button"
            type="button"
            className="task-trigger"
            aria-label={`切换点位：${currentTask?.name ?? "无点位"}`}
            onClick={() => setTaskPickerOpen(true)}
          >
            <TaskIconBadge task={currentTask} testId="current-task-icon" />
            <Box sx={{ minWidth: 0 }}>
              <Typography className="task-trigger-title">
                {currentTask?.name ?? "无点位"}
              </Typography>
              <Typography className="task-trigger-meta">
                {currentTask ? taskMetaLabel(currentTask) : "暂无可切换点位"}
              </Typography>
            </Box>
            <Box className="task-progress">{currentTask ? `${currentTask.completedCount}/${currentTask.totalCount}` : "0/0"}</Box>
            <ExpandMoreIcon className="task-trigger-chevron" />
          </Box>
          <Box className="member-grid">
            {members.map((entry, index) => (
              <Box
                key={entry.member.id}
                className={`member-tile ${entry.completed ? "done" : ""} ${!entry.member.qrImageUrl ? "missing-qr" : ""} ${
                  index === memberIndex ? "active" : ""
                }`}
                onClick={() => setMemberIndex(index)}
                role="button"
                tabIndex={0}
                onKeyDown={(event) => {
                  if (event.key === "Enter" || event.key === " ") setMemberIndex(index);
                }}
              >
                <span>{entry.member.displayName}</span>
                <small>{completionMeta(entry)}</small>
                {canRemove(entry) && (
                  <IconButton
                    className="member-remove"
                    size="small"
                    color="inherit"
                    disabled={removeMember.isPending}
                    onClick={(event) => {
                      event.stopPropagation();
                      confirmRemove(entry);
                    }}
                  >
                    <PersonRemoveIcon fontSize="small" />
                  </IconButton>
                )}
              </Box>
            ))}
          </Box>
          <Stack direction="row" sx={{ gap: 1 }}>
            {currentMemberLive ? (
              <Button
                fullWidth
                variant="contained"
                startIcon={<CheckIcon />}
                disabled={!currentMember || refreshStatus.isPending || isArchived || !isOnline || offlineSnapshot || !currentMemberCanRefresh}
                onClick={() => currentMember && refreshStatus.mutate(currentMember)}
              >
                刷新状态
              </Button>
            ) : (
              <Button
                fullWidth
                variant="contained"
                startIcon={<CheckIcon />}
                disabled={!currentMember || complete.isPending || isArchived || !canToggleCompletion(currentMember)}
                onClick={() => currentMember && toggleCompletion(currentMember)}
              >
                {isArchived
                  ? "已归档"
                  : currentMember?.completed
                    ? `撤销完成 · ${completedLabel}`
                    : `标记 ${currentMember?.member.displayName ?? ""} 完成`}
              </Button>
            )}
          </Stack>
        </Box>
      </Box>

      <Dialog
        open={taskPickerOpen}
        onClose={() => setTaskPickerOpen(false)}
        fullWidth
        maxWidth="sm"
        slotProps={{ paper: { className: "task-picker-paper" } }}
      >
        <DialogTitle className="task-picker-title">
          <Typography component="span" variant="h6" className="task-picker-heading">
            选择点位
          </Typography>
          <IconButton aria-label="关闭点位选择" onClick={() => setTaskPickerOpen(false)} size="small">
            <CloseIcon fontSize="small" />
          </IconButton>
        </DialogTitle>
        <DialogContent sx={{ px: 0, pt: 0, pb: 2 }}>
          <Box className="task-picker-tabs-wrap">
            <Tabs
              value={selectedTaskGroupName}
              onChange={(_event, value: string) => setSelectedTaskGroup(value)}
              variant="scrollable"
              scrollButtons="auto"
              aria-label="点位分组"
              sx={{
                minHeight: 36,
                "& .MuiTab-root": {
                  minHeight: 36,
                  mr: 1,
                  px: 1.75,
                  borderRadius: "999px",
                  bgcolor: "action.hover",
                  color: "text.secondary",
                  fontSize: 13,
                  fontWeight: 900,
                  letterSpacing: 0,
                  textTransform: "none"
                },
                "& .Mui-selected": {
                  bgcolor: "text.primary",
                  color: "background.paper !important"
                },
                "& .MuiTabs-indicator": {
                  display: "none"
                }
              }}
            >
              {groupedTasks.map((group) => (
                <Tab
                  key={group.name}
                  value={group.name}
                  label={
                    <Stack direction="row" sx={{ alignItems: "center", gap: 0.75 }}>
                      <Box component="span" sx={{ width: 8, height: 8, borderRadius: 999, bgcolor: "currentColor", opacity: 0.72 }} />
                      {group.name}
                    </Stack>
                  }
                />
              ))}
            </Tabs>
          </Box>

          <List disablePadding className="task-picker-list">
            {(visibleTaskGroup?.tasks ?? []).map((task) => (
              <ListItemButton
                key={task.id}
                className={`task-picker-card ${task.id === currentTask?.id ? "task-picker-card-selected" : ""}`}
                selected={task.id === currentTask?.id}
                onClick={() => selectTask(task)}
              >
                <TaskIconBadge task={task} testId={`task-icon-${task.id}`} />
                <Stack spacing={0.85} sx={{ minWidth: 0 }}>
                  <Stack direction="row" sx={{ alignItems: "flex-start", justifyContent: "space-between", gap: 1.5 }}>
                    <Box sx={{ minWidth: 0 }}>
                      <Typography className="task-picker-card-name">
                        {task.name}
                      </Typography>
                      <Typography className="task-picker-card-title-text">
                        {task.title || task.name}
                      </Typography>
                    </Box>
                    <Chip
                      size="small"
                      className="task-picker-reward"
                      label={`乐园币 x${task.rewardCoins}`}
                    />
                  </Stack>
                  {task.description && (
                    <Typography className="task-picker-description">
                      {task.description}
                    </Typography>
                  )}
                  <Box sx={{ display: "grid", gridTemplateColumns: "minmax(0, 1fr) auto", alignItems: "center", gap: 1.25, pt: 0.5 }}>
                    <LinearProgress
                      variant="determinate"
                      value={task.totalCount > 0 ? (task.completedCount / task.totalCount) * 100 : 0}
                      className="task-picker-progress"
                    />
                    <Typography className="task-picker-progress-label">
                      {task.completedCount}/{task.totalCount}
                    </Typography>
                  </Box>
                </Stack>
              </ListItemButton>
            ))}
          </List>
        </DialogContent>
      </Dialog>
      <Menu anchorEl={manageAnchor} open={Boolean(manageAnchor)} onClose={() => setManageAnchor(null)}>
        <MenuItem
          onClick={() => {
            setManageAnchor(null);
            setEditOpen(true);
          }}
          disabled={isArchived}
        >
          <ListItemIcon>
            <EditIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>编辑互助组</ListItemText>
        </MenuItem>
        <MenuItem onClick={() => setJoinLocked.mutate(!currentGroup?.joinLocked)} disabled={isArchived || setJoinLocked.isPending}>
          <ListItemIcon>{currentGroup?.joinLocked ? <LockOpenIcon fontSize="small" /> : <LockIcon fontSize="small" />}</ListItemIcon>
          <ListItemText>{currentGroup?.joinLocked ? "解锁加入" : "锁定加入"}</ListItemText>
        </MenuItem>
        <MenuItem onClick={() => void copyInviteLink()}>
          <ListItemIcon>
            <ContentCopyIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>复制邀请链接</ListItemText>
        </MenuItem>
        <MenuItem onClick={() => syncTasks.mutate()} disabled={isArchived || syncTasks.isPending}>
          <ListItemIcon>
            <SyncIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>同步乐园任务</ListItemText>
        </MenuItem>
        <MenuItem onClick={confirmArchive} disabled={isArchived || archiveGroup.isPending}>
          <ListItemIcon>
            <ArchiveIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>归档互助组</ListItemText>
        </MenuItem>
      </Menu>
      <EditGroupDialog
        open={editOpen}
        group={currentGroup ?? null}
        onClose={() => setEditOpen(false)}
        onDone={() => {
          void queryClient.invalidateQueries({ queryKey: ["group", groupId] });
          void queryClient.invalidateQueries({ queryKey: ["groups"] });
        }}
      />
      <Snackbar open={Boolean(copyMessage)} autoHideDuration={1800} message={copyMessage} onClose={() => setCopyMessage("")} />
    </Box>
  );
}

function formatTime(value: string | null) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
}

function currentUser(user?: User): User {
  return user ?? { id: "", displayName: "本机", avatarUrl: "", qrImageUrl: "", qrSource: "uploaded" };
}

function isLiveCompletion(entry: MemberCompletion) {
  return entry.source === "live" || entry.status?.startsWith("live_") === true;
}

function canToggleCompletion(entry: MemberCompletion) {
  return entry.canToggle ?? !isLiveCompletion(entry);
}

function canRefreshCompletion(entry: MemberCompletion) {
  return entry.canRefresh ?? isLiveCompletion(entry);
}

function completionMeta(entry: MemberCompletion) {
  if (!entry.member.qrImageUrl) return "缺二维码";
  if (isLiveCompletion(entry)) {
    const checkedAt = formatTime(entry.liveCheckedAt ?? entry.updatedAt);
    const stale = entry.liveStale ? "，待刷新" : "";
    return checkedAt ? `接口更新 ${checkedAt}${stale}` : `接口${entry.completed ? "已完成" : "未完成"}${stale}`;
  }
  if (entry.completed) return `${formatTime(entry.completedAt)} ${entry.checkedByName}`;
  return "未完成";
}

function taskMetaLabel(task: TaskStatus) {
  return `${task.groupName || "其他点位"} · 乐园币 x${task.rewardCoins}`;
}

function taskIconKind(task?: TaskStatus) {
  const text = `${task?.groupName ?? ""} ${task?.name ?? ""} ${task?.title ?? ""}`;
  if (/舞台|应援|麦克风|演出/.test(text)) return "stage";
  if (/互动|合影|伙伴|社交|集章/.test(text)) return "interaction";
  return "venue";
}

function TaskIconBadge({ task, testId }: { task?: TaskStatus; testId: string }) {
  const kind = taskIconKind(task);
  const config = {
    venue: { Icon: VenueIcon },
    stage: { Icon: StageIcon },
    interaction: { Icon: InteractionIcon }
  }[kind];
  const Icon = config.Icon;
  return (
    <Box data-testid={testId} className={`task-icon-badge task-icon-${kind}`}>
      <Icon sx={{ fontSize: 25 }} />
    </Box>
  );
}

function groupTasksForPicker(tasks: TaskStatus[]) {
  const groups: Array<{ name: string; tasks: TaskStatus[] }> = [];
  for (const task of tasks) {
    const name = task.groupName || "其他点位";
    let group = groups.find((item) => item.name === name);
    if (!group) {
      group = { name, tasks: [] };
      groups.push(group);
    }
    group.tasks.push(task);
  }
  return groups;
}
