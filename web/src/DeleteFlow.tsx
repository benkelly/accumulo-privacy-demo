import { useEffect, useRef, useState } from "react";
import { ApiError, compact, deletePatient, getOldFiles, getPatient, getRaw, type Who } from "./api";

const STEPS = [
  "Delete written",
  "Still on disk?",
  "Compaction",
  "Garbage collection",
];

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

type Props = { who: Who; id: string; onChange: () => void };

// The right to erasure in 4 steps. Each button press runs one step and shows its result.
export default function DeleteFlow({ who, id, onChange }: Props) {
  const [done, setDone] = useState(0);
  const [running, setRunning] = useState(false);
  const [results, setResults] = useState<string[]>([]);
  const [error, setError] = useState("");
  const alive = useRef(false);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);

  const show = (i: number, text: string) =>
    setResults((r) => {
      const next = r.slice(0, i);
      next[i] = text;
      return next;
    });
  const finish = (text: string) => {
    show(done, text);
    setDone((d) => d + 1);
  };

  async function step1() {
    const { deleted } = await deletePatient(who, id);
    try {
      await getPatient(who, id);
      finish(`Delete markers written for ${deleted} cells, but a read still returns data.`);
    } catch (e) {
      if (!(e instanceof ApiError && e.status === 404)) throw e;
      finish(`Delete markers written for ${deleted} cells. A read of ${id} now returns nothing.`);
    }
  }

  async function step2() {
    const raw = await getRaw(who, id);
    const markers = raw.filter((c) => c.deleted).length;
    finish(
      `Yes. The files still hold ${raw.length - markers} cells of ${id}, ` +
        `next to ${markers} delete markers. Anyone with access to the files can read them.`,
    );
  }

  async function step3() {
    await compact(who);
    const raw = await getRaw(who, id);
    // Check HDFS now: the collector may already have run since the compaction.
    const { files, remaining } = await getOldFiles(who);
    const n = files.length;
    const onHdfs =
      remaining === 0
        ? `The garbage collector has already removed the ${n} old files from before the compaction.`
        : remaining === n
          ? `But the ${n} old files from before the compaction are still on HDFS.`
          : `But ${remaining} of the ${n} old files from before the compaction are still on HDFS.`;
    finish(`The files in use now hold ${raw.length || "no"} cells of ${id}. ${onHdfs}`);
  }

  async function step4() {
    // Poll HDFS until the garbage collector has deleted every old file. Give up after 3 minutes.
    for (let i = 0; i < 60 && alive.current; i++) {
      const { files, remaining } = await getOldFiles(who);
      if (remaining === 0) {
        finish(
          `The garbage collector deleted all ${files.length} old files. ${id} is gone from HDFS. ` +
            "Backups, snapshots and the disk itself are outside this demo.",
        );
        return;
      }
      show(3, `${remaining} of ${files.length} old files still on HDFS. Waiting for the collector...`);
      await sleep(3000);
    }
    if (alive.current) throw new Error("The old files are still on HDFS after 3 minutes.");
  }

  async function run() {
    setRunning(true);
    setError("");
    try {
      await [step1, step2, step3, step4][done]();
    } catch (e) {
      const refused = e instanceof ApiError && e.status === 409;
      setError(`${refused ? "Refused" : "Failed"}: ${(e as Error).message}`);
      setResults((r) => r.slice(0, done));
    } finally {
      // Refresh the table, the stored cells and the audit log, also after a refusal.
      onChange();
      if (alive.current) setRunning(false);
    }
  }

  return (
    <section aria-labelledby="erase-heading">
      <h2 id="erase-heading" className="text-xl font-bold">
        Erase {id}
      </h2>
      <p>
        The right to erasure, one step at a time. <code>make seed</code> puts the patient back.
      </p>

      <ol className="mt-2 space-y-2">
        {STEPS.map((title, i) => (
          <li key={title} className="rounded border border-gray-400 p-2">
            <span className="font-semibold">
              {i + 1}. {title}
            </span>{" "}
            <span className="rounded border border-gray-500 px-1">
              {i < done ? "Done" : i === done && running ? "Running" : i === done ? "Next" : "To do"}
            </span>
            {results[i] && <p aria-live="polite">{results[i]}</p>}
          </li>
        ))}
      </ol>

      <p className="mt-2">
        Here the garbage collector runs every 30 seconds. The real default is 5 minutes.
      </p>

      {error && (
        <p role="alert" className="mt-2 rounded border-2 border-red-800 bg-red-50 p-2 text-red-900">
          {error}
        </p>
      )}

      {done < STEPS.length ? (
        <button
          type="button"
          onClick={run}
          disabled={running}
          className="mt-3 w-full rounded-lg border-2 border-red-900 bg-red-800 px-4 py-2 text-lg font-semibold text-white disabled:opacity-60 focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-blue-600"
        >
          {running ? "Running..." : `Run step ${done + 1}: ${STEPS[done]}`}
        </button>
      ) : (
        <p className="mt-3 font-semibold">All 4 steps done.</p>
      )}
    </section>
  );
}
