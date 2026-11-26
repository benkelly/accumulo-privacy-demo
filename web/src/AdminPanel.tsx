import type { AuditEntry, Cell, Who } from "./api";
import DeleteFlow from "./DeleteFlow";

type Props = {
  who: Who;
  selected: string;
  raw: Cell[];
  audit: AuditEntry[];
  onRefresh: () => void;
  onChange: () => void;
};

export default function AdminPanel({ who, selected, raw, audit, onRefresh, onChange }: Props) {
  return (
    <aside aria-label="Admin panel" className="space-y-6 rounded border-2 border-gray-900 p-4">
      {/* A new key resets the flow when you pick another patient. */}
      <DeleteFlow key={selected} who={who} id={selected} onChange={onChange} />

      <section>
        <h2 className="text-xl font-bold">Stored cells for {selected}</h2>
        <p>What the files hold, with the label on each cell.</p>
        {raw.length === 0 ? (
          <p className="mt-2">Nothing is stored for {selected}.</p>
        ) : (
          <ul className="mt-2">
            {raw.map((c, i) => (
              <li key={i} className="border-b border-gray-300 py-2">
                <span className="font-mono font-semibold">
                  {c.family}:{c.qualifier}
                </span>
                <br />
                Label: <span className="font-mono">{c.visibility || "none"}</span>
                <br />
                {c.deleted ? <strong>Delete marker</strong> : <>Value: {c.value}</>}
              </li>
            ))}
          </ul>
        )}
      </section>

      <section>
        <div className="flex items-center justify-between gap-2">
          <h2 className="text-xl font-bold">Audit log</h2>
          <button
            type="button"
            onClick={onRefresh}
            className="rounded border-2 border-gray-900 px-3 py-1 font-semibold focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-blue-600"
          >
            Refresh
          </button>
        </div>
        <p>The last 100 actions, newest first. A restart clears it.</p>
        <ol className="mt-2 space-y-1">
          {audit.map((e, i) => (
            <li key={i} className="border-b border-gray-300 pb-1">
              <time dateTime={e.time}>{new Date(e.time).toLocaleTimeString("en-IE")}</time>{" "}
              <strong>{e.role}</strong>
              {e.patient && ` ${e.patient}`}: {e.action}
              {e.row && ` ${e.row}`}
            </li>
          ))}
        </ol>
      </section>
    </aside>
  );
}
