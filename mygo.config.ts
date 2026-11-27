import { execSync } from "node:child_process";
import { defineConfig } from "mygo-cli";

function resolveVersion(): string {
  const fromEnv =
    process.env.APP_VERSION ||
    (process.env.GITHUB_REF?.startsWith("refs/tags/") ? process.env.GITHUB_REF_NAME : "");
  const clean = (raw: string) => raw.trim().replace(/^v/i, "");
  if (fromEnv) {
    const v = clean(fromEnv);
    if (/^\d+\.\d+\.\d+/.test(v)) return v;
  }
  try {
    const tag = execSync("git describe --tags --abbrev=0", {
      stdio: ["ignore", "pipe", "ignore"],
    }).toString();
    const v = clean(tag);
    if (/^\d+\.\d+\.\d+/.test(v)) return v;
  } catch {
    // Fallback when git metadata or tags are unavailable
  }
  return "0.1.1";
}

export default defineConfig({
  name: "whatsappgui-new",
  identifier: "com.example.whatsappguinew",
  version: resolveVersion(),
  // `mygo dev` runs devCommand and loads devUrl; `mygo build` runs
  // buildCommand and embeds frontendDist into the app.
  devUrl: "http://localhost:5173",
  devCommand: "bun run dev:web",
  buildCommand: "bun run build:web",
  frontendDist: "dist",
  bindings: "src/mygo.ts",
  out: "build",
});
