import { useCallback, useEffect, useState } from "react";
import AdminPanel from "./AdminPanel";
import {
  ROLES,
  getAudit,
  getMe,
  getRaw,
  listPatients,
  type AuditEntry,
  type Cell,
  type Me,
  type Patient,
  type Role,
  type Who,
} from "./api";
import PatientTable from "./PatientTable";

const ROLE_HELP: Record<Role, string> = {
  Admin: "Holds every label.",
  Clinician: "Sees names and clinical data.",
  Legal: "Sees names and legal data.",
  Analyst: "Holds no labels. Sees only public cells.",
  Patient: "Sees their own diagnosis.",
};

const PATIENT_IDS = Array.from({ length: 20 }, (_, i) => `p-${String(i + 1).padStart(4, "0")}`);

// "PATIENT_p0001 to PATIENT_p0020" instead of 20 separate labels.
function labelText(auths: string[]) {
  if (auths.length === 0) return "none";
  const own = auths.filter((a) => a.startsWith("PATIENT_"));
  const rest = auths.filter((a) => !a.startsWith("PATIENT_"));
  if (own.length > 2) rest.push(`${own[0]} to ${own[own.length - 1]}`);
  else rest.push(...own);
  return rest.join(", ");
}

const focus = "focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-blue-600";

export default function App() {
  const [who, setWho] = useState<Who>({ role: "Clinician", patient: "p-0001" });
  const [me, setMe] = useState<Me | null>(null);
  const [patients, setPatients] = useState<Patient[]>([]);
  const [selected, setSelected] = useState("p-0001");
  const [raw, setRaw] = useState<Cell[]>([]);
  const [audit, setAudit] = useState<AuditEntry[]>([]);
  const [error, setError] = useState("");
  const isAdmin = who.role === "Admin";

  const changeWho = (next: Who) => {
    setError("");
    setWho(next);
  };

  useEffect(() => {
    let live = true;
    getMe(who)
      .then((m) => live && setMe(m))
      .catch((e: Error) => live && setError(e.message));
    return () => {
      live = false;
    };
  }, [who]);

  const loadList = useCallback(() => {
    listPatients(who)
      .then(setPatients)
      .catch((e: Error) => setError(e.message));
  }, [who]);

  useEffect(loadList, [loadList]);

  // Raw first, then the audit log, so the log already shows this raw read.
  const loadAdmin = useCallback(() => {
    if (!isAdmin) return;
    getRaw(who, selected)
      .then(setRaw)
      .then(() => getAudit(who))
      .then(setAudit)
      .catch((e: Error) => setError(e.message));
  }, [who, selected, isAdmin]);

  useEffect(loadAdmin, [loadAdmin]);

  return (
    <div className="mx-auto max-w-screen-2xl p-4">
      <header className="mb-6 space-y-4">
        <h1 className="text-3xl font-bold">One table, different readers</h1>
        <p>
          Every cell in Accumulo has a label. Pick a role to read the same patients as that person.
        </p>
        <div role="group" aria-label="Choose a role" className="flex flex-wrap gap-3">
          {ROLES.map((r) => (
            <button
              key={r}
              type="button"
              aria-pressed={who.role === r}
              onClick={() => changeWho({ ...who, role: r })}
              className={`min-w-36 rounded-lg border-2 border-gray-900 px-5 py-3 text-lg font-semibold aria-pressed:bg-gray-900 aria-pressed:text-white ${focus}`}
            >
              {r}
            </button>
          ))}
        </div>
        {who.role === "Patient" && (
          <label className="flex items-center gap-2">
            Patient id
            <select
              value={who.patient}
              onChange={(e) => changeWho({ ...who, patient: e.target.value })}
              className={`rounded border-2 border-gray-900 px-2 py-1 ${focus}`}
            >
              {PATIENT_IDS.map((id) => (
                <option key={id}>{id}</option>
              ))}
            </select>
          </label>
        )}
        <p aria-live="polite" className="rounded bg-gray-100 p-3">
          You are <strong>{who.role}</strong>
          {who.role === "Patient" && ` ${who.patient}`}. {ROLE_HELP[who.role]}
          <br />
          Your labels: <span className="font-mono">{me ? labelText(me.auths) : "..."}</span>
        </p>
        <p>
          Every role sees each patient's ID, county and visits. Those cells have no label, so they
          are public. Labels protect values, not row IDs.
        </p>
      </header>

      {error && (
        <p role="alert" className="mb-4 rounded border-2 border-red-800 bg-red-50 p-3 text-red-900">
          Error: {error}
        </p>
      )}

      <main className={isAdmin ? "grid gap-6 xl:grid-cols-[1fr_24rem]" : ""}>
        <PatientTable
          patients={patients}
          holdsEveryLabel={isAdmin}
          selected={selected}
          onSelect={isAdmin ? setSelected : undefined}
        />
        {isAdmin && (
          <AdminPanel
            who={who}
            selected={selected}
            raw={raw}
            audit={audit}
            onRefresh={loadAdmin}
            onChange={() => {
              loadList();
              loadAdmin();
            }}
          />
        )}
      </main>
    </div>
  );
}
