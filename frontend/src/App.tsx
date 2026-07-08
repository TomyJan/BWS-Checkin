import { CssBaseline, ThemeProvider } from "@mui/material";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import { AuthGate } from "./features/auth/AuthGate";
import { HomePage } from "./features/home/HomePage";
import { appTheme } from "./theme";

const queryClient = new QueryClient();

function GroupPlaceholder() {
  return <main>互助组详情页</main>;
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={appTheme}>
        <CssBaseline />
        <BrowserRouter>
          <AuthGate>
            <Routes>
              <Route path="/" element={<HomePage />} />
              <Route path="/groups/:groupId" element={<GroupPlaceholder />} />
            </Routes>
          </AuthGate>
        </BrowserRouter>
      </ThemeProvider>
    </QueryClientProvider>
  );
}
