import AddIcon from "@mui/icons-material/Add";
import GroupsIcon from "@mui/icons-material/Groups";
import {
  Box,
  Card,
  CardActionArea,
  CardContent,
  Chip,
  Container,
  IconButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Stack,
  Tooltip,
  Typography
} from "@mui/material";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../../api/client";
import type { GroupsResponse, MeResponse } from "../../api/types";
import { QRCodeUpload } from "../profile/QRCodeUpload";

const dayLabel: Record<string, string> = {
  friday: "周五",
  saturday: "周六",
  sunday: "周日"
};

export function HomePage() {
  const navigate = useNavigate();
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const me = useQuery({ queryKey: ["me"], queryFn: () => api<MeResponse>("/me") });
  const groups = useQuery({ queryKey: ["groups"], queryFn: () => api<GroupsResponse>("/groups") });
  const groupItems = groups.data?.groups ?? [];

  return (
    <Box sx={{ minHeight: "100vh", bgcolor: "background.default", py: { xs: 2, md: 4 } }}>
      <Container maxWidth="md">
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
                color="primary"
                size="large"
                sx={{ bgcolor: "primary.main", color: "primary.contrastText", borderRadius: 5 }}
                onClick={(event) => setMenuAnchor(event.currentTarget)}
              >
                <AddIcon />
              </IconButton>
            </Tooltip>
          </Stack>

          {!me.data?.user.qrImageUrl && <QRCodeUpload />}

          <Menu anchorEl={menuAnchor} open={Boolean(menuAnchor)} onClose={() => setMenuAnchor(null)}>
            <MenuItem onClick={() => setMenuAnchor(null)}>
              <ListItemIcon>
                <AddIcon fontSize="small" />
              </ListItemIcon>
              <ListItemText>创建互助组</ListItemText>
            </MenuItem>
            <MenuItem onClick={() => setMenuAnchor(null)}>
              <ListItemIcon>
                <GroupsIcon fontSize="small" />
              </ListItemIcon>
              <ListItemText>加入互助组</ListItemText>
            </MenuItem>
          </Menu>

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
                      <Chip color={group.role === "owner" ? "primary" : "default"} label={group.role === "owner" ? "创建者" : "成员"} />
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
        </Stack>
      </Container>
    </Box>
  );
}
