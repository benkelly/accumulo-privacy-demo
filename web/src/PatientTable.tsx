import type { Patient } from "./api";

const COLUMNS: [key: string, heading: string][] = [
  ["info:name", "Name"],
  ["info:county", "County"],
  ["info:visits", "Visits"],
  ["clinical:diagnosis", "Diagnosis"],
  ["clinical:notes", "Clinical notes"],
  ["legal:case_note", "Case note"],
  ["legal:medical_evidence", "Medical evidence"],
  ["legal:hold", "Legal hold"],
];

// Grey chip for a cell that Accumulo did not return. The word carries the meaning, not the colour.
function Chip({ text }: { text: string }) {
  return (
    <span className="inline-block rounded border border-gray-500 bg-gray-200 px-2 text-gray-900">
      {text}
    </span>
  );
}

function show(key: string, value: string) {
  if (key === "legal:hold") return value === "true" ? "Yes" : "No";
  return value;
}

type Props = {
  patients: Patient[];
  holdsEveryLabel: boolean;
  selected: string;
  onSelect?: (id: string) => void;
};

export default function PatientTable({ patients, holdsEveryLabel, selected, onSelect }: Props) {
  // A role with every label sees every stored cell, so a missing cell does not exist.
  const missing = holdsEveryLabel ? "none" : "hidden";
  return (
    <div className="overflow-x-auto">
      <table className="w-full border-collapse text-left">
        <caption className="mb-2 text-left">
          {patients.length} patients. <Chip text="hidden" /> means Accumulo returned no cell. You
          cannot tell if the cell is secret or empty.
          {holdsEveryLabel && (
            <>
              {" "}
              <Chip text="none" /> means this patient has no such cell.
            </>
          )}
        </caption>
        <thead>
          <tr className="border-b-2 border-gray-900">
            <th scope="col" className="p-2">
              ID
            </th>
            {COLUMNS.map(([key, heading]) => (
              <th key={key} scope="col" className="p-2">
                {heading}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {patients.map((p) => (
            <tr key={p.id} className={`border-b border-gray-300 ${p.id === selected && onSelect ? "bg-blue-50" : ""}`}>
              <th scope="row" className="p-2 font-normal whitespace-nowrap">
                {onSelect ? (
                  <button
                    type="button"
                    aria-pressed={p.id === selected}
                    onClick={() => onSelect(p.id)}
                    className="rounded border-2 border-blue-800 px-2 font-semibold text-blue-900 underline aria-pressed:bg-blue-800 aria-pressed:text-white focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-blue-600"
                  >
                    {p.id}
                  </button>
                ) : (
                  p.id
                )}
              </th>
              {COLUMNS.map(([key]) => (
                <td key={key} className="p-2 align-top">
                  {key in p.cells ? show(key, p.cells[key]) : <Chip text={missing} />}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
