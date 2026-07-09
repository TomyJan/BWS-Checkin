import {
  Alert,
  Box,
  Button,
  Card,
  CardActionArea,
  CardContent,
  Chip,
  IconButton,
  FormControlLabel,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Stack,
  Switch,
  Tooltip,
  Typography
} from "@mui/material";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { api } from "../../api/client";
import type { Group, GroupsResponse, MeResponse } from "../../api/types";
import { AddIcon, GroupsIcon } from "../../icons";
import { UserLayout } from "../../layouts/UserLayout";
import { CreateGroupDialog, JoinGroupDialog } from "../groups/GroupDialogs";

const dayLabel: Record<string, string> = {
  "20260710": "7 月 10 日",
  "20260711": "7 月 11 日",
  "20260712": "7 月 12 日",
  friday: "周五",
  saturday: "周六",
  sunday: "周日"
};

export function HomePage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [joinOpen, setJoinOpen] = useState(Boolean(searchParams.get("invite")));
  const [includeArchived, setIncludeArchived] = useState(false);
  const me = useQuery({ queryKey: ["me"], queryFn: () => api<MeResponse>("/me") });
  const groups = useQuery({
    queryKey: ["groups", includeArchived],
    queryFn: () => api<GroupsResponse>(`/group/list${includeArchived ? "?includeArchived=1" : ""}`)
  });
  const groupItems = groups.data?.groups ?? [];

  function goToGroup(group: Group) {
    navigate(`/groups/${group.id}`);
  }

  return (
    <UserLayout>
      <Stack spacing={3}>
        <Stack direction="row" sx={{ alignItems: "center", justifyContent: "space-between", gap: 2 }}>
          <Box>
            <Typography variant="h4" sx={{ fontWeight: 850 }}>
              我的互助组
            </Typography>
            <Typography color="text.secondary" sx={{ mt: 0.5 }}>
              邀请码就是互助组 ID。创建组时选择活动日期。
            </Typography>
          </Box>
          <Tooltip title="创建或加入互助组">
            <IconButton
              aria-label="创建或加入互助组"
              color="primary"
              size="large"
              sx={{ bgcolor: "primary.main", color: "primary.contrastText", borderRadius: 5, flex: "0 0 auto" }}
              onClick={(event) => setMenuAnchor(event.currentTarget)}
            >
              <AddIcon />
            </IconButton>
          </Tooltip>
        </Stack>

        {!me.data?.user.qrImageUrl && (
          <Alert
            severity="warning"
            action={
              <Button color="inherit" size="small" onClick={() => navigate("/profile")}>
                去上传
              </Button>
            }
          >
            请先上传二维码，互助组成员才能帮你打卡。
          </Alert>
        )}

        <Menu anchorEl={menuAnchor} open={Boolean(menuAnchor)} onClose={() => setMenuAnchor(null)}>
          <MenuItem
            onClick={() => {
              setMenuAnchor(null);
              setCreateOpen(true);
            }}
          >
            <ListItemIcon>
              <AddIcon fontSize="small" />
            </ListItemIcon>
            <ListItemText>创建互助组</ListItemText>
          </MenuItem>
          <MenuItem
            onClick={() => {
              setMenuAnchor(null);
              setJoinOpen(true);
            }}
          >
            <ListItemIcon>
              <GroupsIcon fontSize="small" />
            </ListItemIcon>
            <ListItemText>加入互助组</ListItemText>
          </MenuItem>
        </Menu>

        <Stack direction="row" sx={{ justifyContent: "flex-end" }}>
          <FormControlLabel
            control={<Switch checked={includeArchived} onChange={(event) => setIncludeArchived(event.target.checked)} />}
            label="显示已归档"
          />
        </Stack>

        <Stack spacing={1.5}>
          {groupItems.map((group) => (
            <Card key={group.id} variant="outlined">
              <CardActionArea onClick={() => navigate(`/groups/${group.id}`)}>
                <CardContent>
                  <Stack direction="row" sx={{ alignItems: "center", justifyContent: "space-between", gap: 2 }}>
                    <Box>
                      <Typography variant="h6" sx={{ fontWeight: 800 }}>
                        {group.name || `BW2026 ${dayLabel[group.day] ?? ""}`}
                      </Typography>
                      <Typography color="text.secondary" sx={{ fontSize: 14 }}>
                        ID: {group.id} · {group.memberCount} 人 · {group.taskCount} 个点位
                      </Typography>
                    </Box>
                    <Stack direction="row" sx={{ gap: 1, flexWrap: "wrap", justifyContent: "flex-end" }}>
                      {group.archivedAt && <Chip color="default" label="已归档" />}
                      {group.joinLocked && !group.archivedAt && <Chip color="warning" label="已锁定" />}
                      <Chip color={group.role === "owner" ? "primary" : "default"} label={group.role === "owner" ? "创建者" : "成员"} />
                    </Stack>
                  </Stack>
                </CardContent>
              </CardActionArea>
            </Card>
          ))}
          {!groups.isLoading && groupItems.length === 0 && (
            <Card variant="outlined">
              <CardContent>
                <Typography color="text.secondary">还没有加入互助组。</Typography>
              </CardContent>
            </Card>
          )}
        </Stack>
        <CreateGroupDialog open={createOpen} onClose={() => setCreateOpen(false)} onDone={goToGroup} />
        <JoinGroupDialog
          open={joinOpen}
          onClose={() => setJoinOpen(false)}
          onDone={goToGroup}
          defaultInvite={searchParams.get("invite") ?? ""}
        />
      </Stack>
    </UserLayout>
  );
}
