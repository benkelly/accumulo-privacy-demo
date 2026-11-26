// Calls to the Go API. The role travels in headers, because the demo has no login.

export const ROLES = ["Admin", "Clinician", "Legal", "Analyst", "Patient"] as const;
export type Role = (typeof ROLES)[number];

export type Who = { role: Role; patient: string };
export type Me = { role: Role; patient: string; auths: string[] };
export type Patient = { id: string; cells: Record<string, string> };
export type Cell = {
  row: string;
  family: string;
  qualifier: string;
  visibility: string;
  value: string;
  deleted: boolean;
};
export type AuditEntry = {
  time: string;
  role: Role;
  patient?: string;
  action: string;
  row?: string;
};

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
  }
}

async function call<T>(who: Who, method: string, path: string): Promise<T> {
  const headers: Record<string, string> = { "X-Demo-Role": who.role };
  if (who.role === "Patient") headers["X-Demo-Patient"] = who.patient;
  const res = await fetch(path, { method, headers });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new ApiError(body.error ?? `${method} ${path} failed: ${res.status}`, res.status);
  return body as T;
}

export const getMe = (who: Who) => call<Me>(who, "GET", "/api/me");
export const listPatients = (who: Who) => call<Patient[]>(who, "GET", "/api/patients");
export const getRaw = (who: Who, id: string) => call<Cell[]>(who, "GET", `/api/patients/${id}/raw`);
export const getAudit = (who: Who) => call<AuditEntry[]>(who, "GET", "/api/audit");
export const getPatient = (who: Who, id: string) => call<Patient>(who, "GET", `/api/patients/${id}`);
export const deletePatient = (who: Who, id: string) =>
  call<{ deleted: number }>(who, "DELETE", `/api/patients/${id}`);
export const compact = (who: Who) => call<{ oldFiles: number }>(who, "POST", "/api/compact");
export const getOldFiles = (who: Who) =>
  call<{ files: { path: string; exists: boolean }[]; remaining: number }>(who, "GET", "/api/old-files");
