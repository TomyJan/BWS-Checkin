export interface User {
  id: string;
  displayName: string;
  avatarUrl: string;
  qrImageUrl: string;
  qrSource: "uploaded" | "bilibili_generated";
}

export interface BilibiliAccount {
  mid: string;
  uname: string;
  faceUrl: string;
  cookieExpiresAt?: string | null;
  lastValidatedAt?: string | null;
}

export interface BilibiliAccountResponse {
  bound: boolean;
  account?: BilibiliAccount;
}

export interface BilibiliLoginQRCodeResponse {
  qrcode: {
    url: string;
    qrcodeKey: string;
    expiresAt: string;
    imageDataUrl?: string;
  };
}

export interface BilibiliLoginPollResponse {
  status: "pending_scan" | "pending_confirm" | "expired" | "confirmed" | "failed";
  message?: string;
  account?: BilibiliAccount;
}

export interface Group {
  id: string;
  name: string;
  day: "friday" | "saturday" | "sunday";
  description: string;
  role: "owner" | "member";
  memberCount: number;
  taskCount: number;
  joinLocked: boolean;
  archivedAt: string | null;
}

export interface MeResponse {
  user: User;
}

export interface GroupsResponse {
  groups: Group[] | null;
}

export interface Member {
  id: string;
  displayName: string;
  qrImageUrl: string;
}

export interface MemberCompletion {
  member: Member;
  completed: boolean;
  completedAt: string | null;
  updatedAt: string | null;
  checkedById: string | null;
  checkedByName: string;
}

export interface TaskStatus {
  id: string;
  groupName: string;
  name: string;
  title: string;
  rewardCoins: number;
  description: string;
  sortOrder: number;
  completedCount: number;
  totalCount: number;
  members: MemberCompletion[] | null;
}

export interface GroupResponse {
  group: Group;
}

export interface TasksResponse {
  tasks: TaskStatus[] | null;
}
