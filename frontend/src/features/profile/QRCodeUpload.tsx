import CloudUploadIcon from "@mui/icons-material/CloudUpload";
import { Alert, Button, Stack } from "@mui/material";
import { useQueryClient } from "@tanstack/react-query";
import { api } from "../../api/client";

export function QRCodeUpload() {
  const queryClient = useQueryClient();

  async function upload(file: File) {
    const form = new FormData();
    form.append("file", file);
    await api("/me/qr", { method: "POST", body: form });
    await queryClient.invalidateQueries({ queryKey: ["me"] });
  }

  return (
    <Alert
      severity="warning"
      action={
        <Button component="label" size="small" startIcon={<CloudUploadIcon />}>
          上传
          <input
            hidden
            accept="image/png,image/jpeg,image/webp"
            type="file"
            onChange={(event) => {
              const file = event.target.files?.[0];
              if (file) void upload(file);
            }}
          />
        </Button>
      }
    >
      <Stack>请先上传二维码，互助组成员才能帮你打卡。</Stack>
    </Alert>
  );
}
