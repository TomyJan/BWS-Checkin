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
  id: number;
  displayName: string;
  qrImageUrl: string;
}

export interface MemberCompletion {
  member: Member;
  completed: boolean;
  completedAt: string | null;
  updatedAt: string | null;
  checkedById: number | null;
  checkedByName: string;
}

export interface TaskStatus {
  id: string;
  name: string;
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
