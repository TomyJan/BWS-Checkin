import { qrImageURL } from "../api/qr";
import type { Group, TaskStatus } from "../api/types";

export interface GroupSnapshot {
  group: Group;
  tasks: TaskStatus[];
  savedAt: string;
}

const SNAPSHOT_PREFIX = "bws:group-snapshot:";
const QR_CACHE = "bws-qr-v1";

export function loadGroupSnapshot(groupId: string): GroupSnapshot | null {
  const raw = localStorage.getItem(SNAPSHOT_PREFIX + groupId);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as GroupSnapshot;
  } catch {
    return null;
  }
}

export async function saveGroupSnapshot(snapshot: GroupSnapshot) {
  localStorage.setItem(SNAPSHOT_PREFIX + snapshot.group.id, JSON.stringify(snapshot));
  await preloadQRCodeImages(snapshot.tasks);
}

export async function preloadQRCodeImages(tasks: TaskStatus[]) {
  if (!("caches" in window)) return;
  const urls = new Set<string>();
  for (const task of tasks) {
    for (const entry of task.members ?? []) {
      const url = qrImageURL(entry.member);
      if (url) urls.add(url);
    }
  }
  const cache = await caches.open(QR_CACHE);
  await Promise.allSettled(
    Array.from(urls).map(async (url) => {
      const response = await fetch(url, { cache: "reload", credentials: "include" });
      if (response.ok) await cache.put(url, response.clone());
    })
  );
}
