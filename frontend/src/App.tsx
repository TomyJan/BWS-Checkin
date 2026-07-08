import { CssBaseline, ThemeProvider } from "@mui/material";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import { appTheme } from "./theme";

const queryClient = new QueryClient();

function HomePlaceholder() {
  return <main>BWS Checkin 首页</main>;
}

function GroupPlaceholder() {
  return <main>互助组详情页</main>;
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={appTheme}>
        <CssBaseline />
        <BrowserRouter>
          <Routes>
            <Route path="/" element={<HomePlaceholder />} />
            <Route path="/groups/:groupId" element={<GroupPlaceholder />} />
          </Routes>
        </BrowserRouter>
      </ThemeProvider>
    </QueryClientProvider>
  );
}
