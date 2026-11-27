export interface FileKindInfo {
  badge: string;
  label: string;
  tone: string;
}

export function joinPath(dir: string, file: string): string {
  if (!dir) return file;
  const sep = dir.includes("/") && !dir.includes("\\") ? "/" : "\\";
  return dir.endsWith("\\") || dir.endsWith("/") ? `${dir}${file}` : `${dir}${sep}${file}`;
}

export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  return `${value >= 10 || unit === 0 ? Math.round(value) : value.toFixed(1)} ${units[unit]}`;
}

export function fileKind(fileName: string): FileKindInfo {
  const ext = fileName.includes(".") ? fileName.split(".").pop()?.toLowerCase() ?? "" : "";
  if (["png", "jpg", "jpeg", "gif", "webp", "bmp", "svg"].includes(ext)) {
    return { badge: ext.toUpperCase() || "IMG", label: "Gambar", tone: "image" };
  }
  if (ext === "pdf") return { badge: "PDF", label: "Dokumen PDF", tone: "pdf" };
  if (["mp4", "mkv", "mov", "webm", "avi"].includes(ext)) {
    return { badge: ext.toUpperCase(), label: "Video", tone: "video" };
  }
  if (["mp3", "ogg", "opus", "wav", "m4a", "aac"].includes(ext)) {
    return { badge: ext.toUpperCase(), label: "Audio", tone: "audio" };
  }
  if (["zip", "rar", "7z", "tar", "gz"].includes(ext)) {
    return { badge: ext.toUpperCase(), label: "Arsip", tone: "archive" };
  }
  if (["doc", "docx", "xls", "xlsx", "ppt", "pptx", "txt", "csv"].includes(ext)) {
    return { badge: ext.toUpperCase(), label: "Dokumen", tone: "doc" };
  }
  return { badge: ext ? ext.slice(0, 4).toUpperCase() : "FILE", label: "Berkas", tone: "file" };
}
