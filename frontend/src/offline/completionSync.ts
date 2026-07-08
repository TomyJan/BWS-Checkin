import { api } from "../api/client";
import type { TaskStatus, User } from "../api/types";

export interface PendingCompletion {
  id: string;
  groupId: string;
  taskId: string;
  userId: number;
  completed: boolean;
  updatedAt: string;
}

const QUEUE_KEY = "bws:pending-completions";

export function queueCompletion(action: Omit<PendingCompletion, "id">) {
  const pending = readQueue().filter(
    (item) => !(item.groupId === action.groupId && item.taskId === action.taskId && item.userId === action.userId)
  );
  pending.push({ ...action, id: crypto.randomUUID() });
  writeQueue(pending);
}

export async function trySyncCompletion(action: Omit<PendingCompletion, "id">) {
  await api(action.completed ? "/task/complete" : "/task/uncomplete", {
    method: "POST",
    body: JSON.stringify({
      groupId: action.groupId,
      taskId: action.taskId,
      userId: action.userId,
      updatedAt: action.updatedAt
    })
  });
}

export async function flushCompletionQueue() {
  const pending = readQueue().sort((a, b) => a.updatedAt.localeCompare(b.updatedAt));
  const remaining: PendingCompletion[] = [];
  for (const action of pending) {
    try {
      await trySyncCompletion(action);
    } catch {
      remaining.push(action);
    }
  }
  writeQueue(remaining);
  return remaining.length;
}

export function applyCompletionToTasks(tasks: TaskStatus[], action: Omit<PendingCompletion, "id">, checkedBy: User): TaskStatus[] {
  return tasks.map((task) => {
    if (task.id !== action.taskId) return task;
    let completedCount = 0;
    const members = (task.members ?? []).map((entry) => {
      if (entry.member.id !== action.userId) {
        if (entry.completed) completedCount += 1;
        return entry;
      }
      const next = {
        ...entry,
        completed: action.completed,
        completedAt: action.completed ? action.updatedAt : entry.completedAt,
        updatedAt: action.updatedAt,
        checkedById: checkedBy.id,
        checkedByName: checkedBy.displayName
      };
      if (next.completed) completedCount += 1;
      return next;
    });
    return { ...task, members, completedCount };
  });
}

function readQueue(): PendingCompletion[] {
  const raw = localStorage.getItem(QUEUE_KEY);
  if (!raw) return [];
  try {
    return JSON.parse(raw) as PendingCompletion[];
  } catch {
    return [];
  }
}

function writeQueue(items: PendingCompletion[]) {
  localStorage.setItem(QUEUE_KEY, JSON.stringify(items));
}
