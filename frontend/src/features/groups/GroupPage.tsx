import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import CheckIcon from "@mui/icons-material/Check";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import MoreVertIcon from "@mui/icons-material/MoreVert";
import NavigateBeforeIcon from "@mui/icons-material/NavigateBefore";
import NavigateNextIcon from "@mui/icons-material/NavigateNext";
import PersonRemoveIcon from "@mui/icons-material/PersonRemove";
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
  Typography
} from "@mui/material";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { api } from "../../api/client";
import type { GroupResponse, MeResponse, MemberCompletion, TasksResponse, TaskStatus, User } from "../../api/types";
import {
  applyCompletionToTasks,
  flushCompletionQueue,
  queueCompletion,
  trySyncCompletion,
  type PendingCompletion
} from "../../offline/completionSync";
import { loadGroupSnapshot, saveGroupSnapshot } from "../../offline/groupSnapshot";

export function GroupPage() {
  const { groupId = "" } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [uiVisible, setUiVisible] = useState(true);
  const [taskIndex, setTaskIndex] = useState(0);
  const [memberIndex, setMemberIndex] = useState(0);
  const [taskPickerOpen, setTaskPickerOpen] = useState(false);
  const [offlineSnapshot, setOfflineSnapshot] = useState(false);
  const [manageAnchor, setManageAnchor] = useState<HTMLElement | null>(null);
  const [copyMessage, setCopyMessage] = useState("");
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
  const members = currentTask?.members ?? [];
  const currentMember = members[Math.min(memberIndex, Math.max(members.length - 1, 0))];
  const isOwner = group.data?.group.role === "owner";

  const complete = useMutation({
    mutationFn: async (action: Omit<PendingCompletion, "id">) => {
      try {
        await trySyncCompletion(action);
        return { action, synced: true };
      } catch {
        queueCompletion(action);
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
    onSuccess: ({ synced }) => {
      if (synced) void queryClient.invalidateQueries({ queryKey: ["groupTasks", groupId] });
    }
  });

  const removeMember = useMutation({
    mutationFn: async (userId: number) => {
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
    if (!currentTask) return;
    complete.mutate({
      groupId,
      taskId: currentTask.id,
      userId: entry.member.id,
      completed: !entry.completed,
      updatedAt: new Date().toISOString()
    });
  }

  async function copyInviteLink() {
    const invite = `${window.location.origin}/?invite=${encodeURIComponent(groupId)}`;
    await navigator.clipboard.writeText(invite);
    setCopyMessage("已复制邀请链接");
    setManageAnchor(null);
  }

  function canRemove(entry: MemberCompletion) {
    return isOwner && entry.member.id !== me?.user.id;
  }

  useEffect(() => {
    const groupValue = group.data?.group;
    if (!groupValue || !tasksQuery.data?.tasks) return;
    void saveGroupSnapshot({ group: groupValue, tasks: tasksQuery.data.tasks, savedAt: new Date().toISOString() });
  }, [group.data?.group, tasksQuery.data?.tasks]);

  useEffect(() => {
    async function syncPending() {
      const remaining = await flushCompletionQueue();
      if (remaining === 0) {
        await queryClient.invalidateQueries({ queryKey: ["groupTasks", groupId] });
      }
    }
    if (navigator.onLine) void syncPending();
    window.addEventListener("online", syncPending);
    return () => window.removeEventListener("online", syncPending);
  }, [groupId, queryClient]);

  const completedLabel = useMemo(() => {
    if (!currentMember?.completedAt) return "";
    const date = new Date(currentMember.completedAt);
    const time = Number.isNaN(date.getTime())
      ? currentMember.completedAt
      : date.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
    return `${time} ${currentMember.checkedByName}`;
  }, [currentMember]);

  if (loading) {
    return (
      <Stack sx={{ minHeight: "100vh", alignItems: "center", justifyContent: "center" }}>
        <CircularProgress />
      </Stack>
    );
  }

  return (
    <Box className={`qr-viewer ${uiVisible ? "ui-visible" : "ui-hidden"}`} onClick={() => setUiVisible((visible) => !visible)}>
      <Box className="qr-photo-stage">
        {currentMember?.member.qrImageUrl ? (
          <img className="qr-photo-img" src={currentMember.member.qrImageUrl} alt={`${currentMember.member.displayName} 的二维码`} />
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
                {group.data?.group.name}
              </Typography>
              <Typography sx={{ color: "rgba(255,255,255,.72)", fontSize: 13 }}>
                正在查看 {currentMember?.member.displayName ?? "-"} 的二维码
              </Typography>
            </Box>
          </Stack>
          <Stack direction="row" sx={{ alignItems: "center", gap: 1 }}>
            {offlineSnapshot && <Chip label="离线快照" color="warning" size="small" />}
            {isOwner && (
              <IconButton color="inherit" onClick={(event) => setManageAnchor(event.currentTarget)}>
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
          <Stack direction="row" sx={{ alignItems: "center", justifyContent: "space-between", gap: 2 }}>
            <Button color="inherit" endIcon={<ExpandMoreIcon />} onClick={() => setTaskPickerOpen(true)} sx={{ px: 0, fontSize: 18, fontWeight: 850 }}>
              {currentTask?.name ?? "无点位"}
            </Button>
            <Box className="task-progress">{currentTask ? `${currentTask.completedCount}/${currentTask.totalCount}` : "0/0"}</Box>
          </Stack>
          <LinearProgress
            variant="determinate"
            value={currentTask && currentTask.totalCount > 0 ? (currentTask.completedCount / currentTask.totalCount) * 100 : 0}
            sx={{ borderRadius: 999 }}
          />
          <Box className="member-grid">
            {members.map((entry, index) => (
              <Box
                key={entry.member.id}
                className={`member-tile ${entry.completed ? "done" : ""} ${index === memberIndex ? "active" : ""}`}
                onClick={() => setMemberIndex(index)}
                role="button"
                tabIndex={0}
                onKeyDown={(event) => {
                  if (event.key === "Enter" || event.key === " ") setMemberIndex(index);
                }}
              >
                <span>{entry.member.displayName}</span>
                <small>{entry.completed ? `${formatTime(entry.completedAt)} ${entry.checkedByName}` : "未完成"}</small>
                {canRemove(entry) && (
                  <IconButton
                    className="member-remove"
                    size="small"
                    color="inherit"
                    disabled={removeMember.isPending}
                    onClick={(event) => {
                      event.stopPropagation();
                      removeMember.mutate(entry.member.id);
                    }}
                  >
                    <PersonRemoveIcon fontSize="small" />
                  </IconButton>
                )}
              </Box>
            ))}
          </Box>
          <Stack direction="row" sx={{ gap: 1 }}>
            <Button
              fullWidth
              variant="contained"
              startIcon={<CheckIcon />}
              disabled={!currentMember || complete.isPending}
              onClick={() => currentMember && toggleCompletion(currentMember)}
            >
              {currentMember?.completed ? `撤销完成 · ${completedLabel}` : `标记 ${currentMember?.member.displayName ?? ""} 完成`}
            </Button>
          </Stack>
        </Box>
      </Box>

      <Dialog open={taskPickerOpen} onClose={() => setTaskPickerOpen(false)} fullWidth maxWidth="xs">
        <DialogTitle>选择点位</DialogTitle>
        <DialogContent>
          <List>
            {tasks.map((task) => (
              <ListItemButton key={task.id} selected={task.id === currentTask?.id} onClick={() => selectTask(task)}>
                <ListItemText primary={task.name} secondary={`${task.completedCount}/${task.totalCount}`} />
              </ListItemButton>
            ))}
          </List>
        </DialogContent>
      </Dialog>
      <Menu anchorEl={manageAnchor} open={Boolean(manageAnchor)} onClose={() => setManageAnchor(null)}>
        <MenuItem onClick={() => void copyInviteLink()}>
          <ListItemIcon>
            <ContentCopyIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>复制邀请链接</ListItemText>
        </MenuItem>
      </Menu>
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
  return user ?? { id: 0, displayName: "本机", avatarUrl: "", qrImageUrl: "" };
}
