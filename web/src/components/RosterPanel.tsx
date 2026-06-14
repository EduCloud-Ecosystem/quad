import { useEffect, useState } from "react";
import { api, type RosterEntry } from "../api";
import { Button, StatusChip, Empty, type Notify } from "./ui";

export function RosterPanel({ classroomID, notify }: { classroomID: string; notify: Notify }) {
  const [roster, setRoster] = useState<RosterEntry[] | null>(null);
  const [username, setUsername] = useState("");
  const [busy, setBusy] = useState(false);

  async function load() {
    try {
      setRoster(await api.listRoster(classroomID));
    } catch (e) {
      notify(e instanceof Error ? e.message : String(e), "err");
    }
  }

  useEffect(() => {
    setRoster(null);
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [classroomID]);

  async function add() {
    const u = username.trim();
    if (!u) return;
    setBusy(true);
    try {
      await api.addRoster(classroomID, { username: u });
      setUsername("");
      notify(`Added ${u} to the roster`);
      void load();
    } catch (e) {
      notify(e instanceof Error ? e.message : String(e), "err");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="section">
      <div className="section-head">
        <span className="section-title">
          Roster
          {roster && <span className="count">{roster.length}</span>}
        </span>
      </div>

      <div className="form-row" style={{ marginBottom: 14 }}>
        <input
          className="input"
          placeholder="Git username (e.g. octocat)"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") void add();
          }}
          style={{ minWidth: 240 }}
        />
        <Button variant="primary" disabled={busy} onClick={() => void add()}>
          Add student
        </Button>
        <span className="muted small">No names or emails — only the username is stored.</span>
      </div>

      {roster === null ? (
        <p className="muted small">Loading roster…</p>
      ) : roster.length === 0 ? (
        <Empty>No students invited yet.</Empty>
      ) : (
        <table className="table">
          <thead>
            <tr>
              <th>Username</th>
              <th>Status</th>
              <th>Claimed</th>
            </tr>
          </thead>
          <tbody>
            {roster.map((r) => (
              <tr key={r.id}>
                <td className="mono">{r.host_username}</td>
                <td>
                  <StatusChip status={r.status} />
                </td>
                <td className="mono muted">{r.claimed_at ? new Date(r.claimed_at).toLocaleString() : "—"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
