export interface User {
  id: number;
  displayName: string;
  avatarUrl: string;
  qrImageUrl: string;
}

export interface Group {
  id: string;
  name: string;
  day: "friday" | "saturday" | "sunday";
  description: string;
  role: "owner" | "member";
  memberCount: number;
  taskCount: number;
}
