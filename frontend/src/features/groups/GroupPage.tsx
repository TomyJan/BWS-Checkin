import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import CheckIcon from "@mui/icons-material/Check";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import NavigateBeforeIcon from "@mui/icons-material/NavigateBefore";
import NavigateNextIcon from "@mui/icons-material/NavigateNext";
import {
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogContent,
  DialogTitle,
  IconButton,
  LinearProgress,
  List,
  ListItemButton,
  ListItemText,
  Stack,
  Typography
} from "@mui/material";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { api } from "../../api/client";
import type { GroupResponse, MemberCompletion, TasksResponse, TaskStatus } from "../../api/types";

export function GroupPage() {
  const { groupId = "" } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [uiVisible, setUiVisible] = useState(true);
  const [taskIndex, setTaskIndex] = useState(0);
  const [memberIndex, setMemberIndex] = useState(0);
  const [taskPickerOpen, setTaskPickerOpen] = useState(false);

  const group = useQuery({ queryKey: ["group", groupId], queryFn: () => api<GroupResponse>(`/groups/${groupId}`), enabled: Boolean(groupId) });
  const tasksQuery = useQuery({
    queryKey: ["groupTasks", groupId],
    queryFn: () => api<TasksResponse>(`/groups/${groupId}/tasks`),
    enabled: Boolean(groupId)
  });

  const tasks = tasksQuery.data?.tasks ?? [];
  const currentTask = tasks[Math.min(taskIndex, Math.max(tasks.length - 1, 0))];
  const members = currentTask?.members ?? [];
  const currentMember = members[Math.min(memberIndex, Math.max(members.length - 1, 0))];

  const complete = useMutation({
    mutationFn: (entry: MemberCompletion) =>
      api(`/groups/${groupId}/tasks/${currentTask.id}/members/${entry.member.id}/complete`, { method: "POST" }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["groupTasks", groupId] })
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
              <button
                key={entry.member.id}
                className={`member-tile ${entry.completed ? "done" : ""} ${index === memberIndex ? "active" : ""}`}
                onClick={() => setMemberIndex(index)}
              >
                <span>{entry.member.displayName}</span>
                <small>{entry.completed ? `${formatTime(entry.completedAt)} ${entry.checkedByName}` : "未完成"}</small>
              </button>
            ))}
          </Box>
          <Stack direction="row" sx={{ gap: 1 }}>
            <Button
              fullWidth
              variant="contained"
              startIcon={<CheckIcon />}
              disabled={!currentMember || currentMember.completed || complete.isPending}
              onClick={() => currentMember && complete.mutate(currentMember)}
            >
              {currentMember?.completed ? completedLabel : `标记 ${currentMember?.member.displayName ?? ""} 完成`}
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
    </Box>
  );
}

function formatTime(value: string | null) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
}
