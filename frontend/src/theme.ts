import { createTheme } from "@mui/material/styles";

export const appTheme = createTheme({
  cssVariables: {
    colorSchemeSelector: "media"
  },
  colorSchemes: {
    light: {
      palette: {
        primary: { main: "#006b5f" },
        background: { default: "#f7f9fc", paper: "#ffffff" }
      }
    },
    dark: {
      palette: {
        primary: { main: "#75d8c8" },
        background: { default: "#0f1418", paper: "#181d22" }
      }
    }
  },
  shape: {
    borderRadius: 22
  },
  components: {
    MuiButton: {
      styleOverrides: {
        root: { borderRadius: 22, textTransform: "none", fontWeight: 700 }
      }
    },
    MuiDialog: {
      styleOverrides: {
        paper: { borderRadius: 28 }
      }
    },
    MuiCard: {
      styleOverrides: {
        root: { borderRadius: 24 }
      }
    }
  }
});
