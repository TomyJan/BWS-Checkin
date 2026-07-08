import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Stack,
  TextField,
  ToggleButton,
  ToggleButtonGroup
} from "@mui/material";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { api } from "../../api/client";
import type { Group } from "../../api/types";

export interface CreateGroupValues {
  id: string;
  name: string;
  day: "friday" | "saturday" | "sunday";
  description: string;
}

interface DialogProps {
  open: boolean;
  onClose: () => void;
  onDone: (group: Group) => void;
}

export function CreateGroupDialog({ open, onClose, onDone }: DialogProps) {
  const queryClient = useQueryClient();
  const [values, setValues] = useState<CreateGroupValues>({
    id: "",
    name: "",
    day: "friday",
    description: ""
  });

  const createGroup = useMutation({
    mutationFn: () =>
      api<{ group: Group }>("/group/create", {
        method: "POST",
        body: JSON.stringify(values)
      }),
    onSuccess: async ({ group }) => {
      await queryClient.invalidateQueries({ queryKey: ["groups"] });
      onDone(group);
      onClose();
    }
  });

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>创建互助组</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ pt: 1 }}>
          <TextField label="名称" value={values.name} onChange={(event) => setValues({ ...values, name: event.target.value })} />
          <TextField label="ID / 邀请码" value={values.id} onChange={(event) => setValues({ ...values, id: event.target.value })} />
          <ToggleButtonGroup
            exclusive
            fullWidth
            color="primary"
            value={values.day}
            onChange={(_, day) => {
              if (day) setValues({ ...values, day });
            }}
          >
            <ToggleButton value="friday">周五</ToggleButton>
            <ToggleButton value="saturday">周六</ToggleButton>
            <ToggleButton value="sunday">周日</ToggleButton>
          </ToggleButtonGroup>
          <TextField
            multiline
            minRows={3}
            label="说明"
            value={values.description}
            onChange={(event) => setValues({ ...values, description: event.target.value })}
          />
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>取消</Button>
        <Button disabled={!values.id || !values.name || createGroup.isPending} variant="contained" onClick={() => createGroup.mutate()}>
          创建
        </Button>
      </DialogActions>
    </Dialog>
  );
}

interface EditGroupDialogProps extends DialogProps {
  group: Group | null;
}

export function EditGroupDialog({ open, onClose, onDone, group }: EditGroupDialogProps) {
  const queryClient = useQueryClient();
  const [values, setValues] = useState<CreateGroupValues>({
    id: "",
    name: "",
    day: "friday",
    description: ""
  });

  useEffect(() => {
    if (!group || !open) return;
    setValues({
      id: group.id,
      name: group.name,
      day: group.day,
      description: group.description
    });
  }, [group, open]);

  const updateGroup = useMutation({
    mutationFn: () =>
      api<{ group: Group }>("/group/update", {
        method: "POST",
        body: JSON.stringify({
          groupId: values.id,
          name: values.name,
          day: values.day,
          description: values.description
        })
      }),
    onSuccess: async ({ group }) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["groups"] }),
        queryClient.invalidateQueries({ queryKey: ["group", group.id] })
      ]);
      onDone(group);
      onClose();
    }
  });

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>编辑互助组</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ pt: 1 }}>
          <TextField label="名称" value={values.name} onChange={(event) => setValues({ ...values, name: event.target.value })} />
          <TextField label="ID / 邀请码" value={values.id} disabled />
          <ToggleButtonGroup
            exclusive
            fullWidth
            color="primary"
            value={values.day}
            onChange={(_, day) => {
              if (day) setValues({ ...values, day });
            }}
          >
            <ToggleButton value="friday">周五</ToggleButton>
            <ToggleButton value="saturday">周六</ToggleButton>
            <ToggleButton value="sunday">周日</ToggleButton>
          </ToggleButtonGroup>
          <TextField
            multiline
            minRows={3}
            label="说明"
            value={values.description}
            onChange={(event) => setValues({ ...values, description: event.target.value })}
          />
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>取消</Button>
        <Button disabled={!values.id || !values.name || updateGroup.isPending} variant="contained" onClick={() => updateGroup.mutate()}>
          保存
        </Button>
      </DialogActions>
    </Dialog>
  );
}

interface JoinGroupDialogProps extends DialogProps {
  defaultInvite?: string;
}

export function JoinGroupDialog({ open, onClose, onDone, defaultInvite = "" }: JoinGroupDialogProps) {
  const queryClient = useQueryClient();
  const [groupId, setGroupId] = useState(defaultInvite);

  useEffect(() => {
    if (open) setGroupId(defaultInvite);
  }, [defaultInvite, open]);

  const joinGroup = useMutation({
    mutationFn: () =>
      api<{ group: Group }>("/group/join", {
        method: "POST",
        body: JSON.stringify({ groupId })
      }),
    onSuccess: async ({ group }) => {
      await queryClient.invalidateQueries({ queryKey: ["groups"] });
      onDone(group);
      onClose();
    }
  });

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="xs">
      <DialogTitle>加入互助组</DialogTitle>
      <DialogContent>
        <TextField
          autoFocus
          fullWidth
          sx={{ mt: 1 }}
          label="邀请码 / 组 ID"
          value={groupId}
          onChange={(event) => setGroupId(event.target.value)}
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>取消</Button>
        <Button disabled={!groupId || joinGroup.isPending} variant="contained" onClick={() => joinGroup.mutate()}>
          加入
        </Button>
      </DialogActions>
    </Dialog>
  );
}
