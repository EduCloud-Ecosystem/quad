import type { ButtonHTMLAttributes, ReactNode } from "react";

export type Notify = (msg: string, kind?: "ok" | "err") => void;

type Variant = "default" | "primary" | "danger" | "ghost";

export function Button({
  variant = "default",
  small,
  className,
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant; small?: boolean }) {
  const cls = [
    "btn",
    variant === "primary" ? "btn-primary" : "",
    variant === "danger" ? "btn-danger" : "",
    variant === "ghost" ? "btn-ghost" : "",
    small ? "btn-sm" : "",
    className ?? "",
  ]
    .filter(Boolean)
    .join(" ");
  return <button className={cls} {...rest} />;
}

export function Field({
  label,
  children,
  full,
}: {
  label: string;
  children: ReactNode;
  full?: boolean;
}) {
  return (
    <label className={"field" + (full ? " span-2" : "")}>
      <span>{label}</span>
      {children}
    </label>
  );
}

// Maps a server status string to a chip style.
const CHIP: Record<string, string> = {
  active: "chip-ok",
  succeeded: "chip-ok",
  invited: "chip-warn",
  provisioning: "chip-info",
  running: "chip-info",
  locked: "chip-slate",
  removed: "chip-muted",
  failed: "chip-danger",
  error: "chip-danger",
};

export function StatusChip({ status }: { status: string }) {
  const cls = CHIP[status] ?? "chip-muted";
  return <span className={`chip ${cls}`}>{status || "unknown"}</span>;
}

export function Modal({
  title,
  subtitle,
  onClose,
  children,
}: {
  title: string;
  subtitle?: string;
  onClose: () => void;
  children: ReactNode;
}) {
  return (
    <div className="overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h2>{title}</h2>
        {subtitle && <p className="modal-sub">{subtitle}</p>}
        {children}
      </div>
    </div>
  );
}

export function Empty({ children }: { children: ReactNode }) {
  return <div className="empty">{children}</div>;
}
