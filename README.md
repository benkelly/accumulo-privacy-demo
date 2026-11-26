# accumulo-privacy-demo

A small demo of Apache Accumulo cell-level security, with fake medical and
legal records. The same row, read by different people, returns different
cells.

Work in progress. It goes with the blog post "Privacy at the storage layer:
what Accumulo taught me about data lakes" on
[blog.benjamin.ie](https://blog.benjamin.ie).

## Run it

You need Docker. Then run:

```
make up
```

This builds the images, starts the stack, waits until Accumulo is ready and
writes 20 fake patients. Then open http://localhost:5173. Use `make down` to
stop it, and `make clean` to stop it and delete all data.

`make seed` writes the 20 patients again. So does every `make up`. This puts
back any patient that you deleted. The seed then runs a compaction, so the
files hold one version of each cell.

`make unit` runs the unit tests in Docker. You do not need Java or Go
installed.

The API is on `http://localhost:8080`. Try it:

```
curl -H "X-Demo-Role: Analyst" http://localhost:8080/api/patients/p-0001
curl -H "X-Demo-Role: Clinician" http://localhost:8080/api/patients/p-0001
```

The Analyst sees only the county and the visits. The Clinician also sees the
name, the diagnosis and the notes.

## The Accumulo stack

| Service   | Image                               | Versions                         |
|-----------|-------------------------------------|----------------------------------|
| zookeeper | `zookeeper:3.8.8-jre-17` (official) | ZooKeeper 3.8.8                  |
| hdfs      | built from `accumulo/Dockerfile`    | Hadoop 3.3.6                     |
| accumulo  | built from `accumulo/Dockerfile`    | Accumulo 2.1.6, Java 17          |

Why build the image here? There is no maintained arm64 image for Accumulo 2.1
or for Hadoop. The `apache/accumulo` image is for Accumulo 4.0 only. So the
Dockerfile downloads the Apache release files and checks their SHA-512 sums.
One image runs both HDFS and Accumulo, with a different start script each.

On first start, `accumulo/init.sh` creates the `patients` table and a
`gateway` user that holds every label. The table is created with
`createtable -evc`, so Accumulo refuses a write with a label that the writer
does not hold. A typo such as `CLINCIAN` fails at once, and cannot hide a
cell from everyone. If your data is older than this change, run `make clean`
one time, because `-evc` applies only when the table is created. The health check passes only after
that step, and only when the table can serve a scan.

`make down` stops Accumulo cleanly with `accumulo admin stopAll`. Then the
next start has no write-ahead log to replay. If Accumulo is killed instead,
the next start is slower, because Accumulo replays the log first.

## The gateway

Accumulo's only full client is Java. So `gateway/` is a small Java service
that holds all the Accumulo code, in `Store.java`. It is internal: Compose
does not publish its port. It has these endpoints:

| Endpoint                        | What it does                                        |
|---------------------------------|-----------------------------------------------------|
| `POST /write`                   | Writes one cell with a visibility label             |
| `GET /scan?row=...&auths=A,B`   | Returns the cells that those auths can see          |
| `POST /delete`                  | Writes a delete marker for every cell in a row      |
| `POST /compact`                 | Runs a full compaction and waits for it             |
| `GET /raw?row=...`              | Reads the row from the table files, with no filters |

The gateway user holds every label, but `/scan` always uses the auths in
the request. A scan with no auths returns only the public cells.

`/raw` shows the erasure gap. After a delete, a scan returns nothing, but
the old cells are still in the files, next to the delete markers. Only a
full compaction writes new files without them.

## The API

`api/` is a Go service on port 8080 that uses only the standard library. It
holds the demo's rules: which role gets which labels, who can delete, and the
audit log. It calls the gateway with the caller's labels.

The `X-Demo-Role` header sets the role. The Patient role also needs
`X-Demo-Patient`, for example `p-0001`. This is a demo with no login, so
anyone can send any role. Compose binds the port to localhost only.

| Role      | Labels                                   |
|-----------|------------------------------------------|
| Admin     | PII, CLINICIAN, LEGAL, PATIENT_p0001 to PATIENT_p0020 |
| Clinician | PII, CLINICIAN                           |
| Legal     | PII, LEGAL                               |
| Analyst   | none                                     |
| Patient   | PATIENT_<id> only                        |

Accumulo labels have no wildcards, so Admin gets one `PATIENT_` label per
patient. This table is `roleAuths` in `api/roles.go`.

The data has these labels:

| Cell                    | Label                      |
|-------------------------|----------------------------|
| `info:name`             | `PII`                      |
| `info:county`           | none (public)              |
| `info:visits`           | none (public)              |
| `clinical:diagnosis`    | `CLINICIAN\|PATIENT_<id>`  |
| `clinical:notes`        | `CLINICIAN`                |
| `legal:case_note`       | `LEGAL`                    |
| `legal:medical_evidence`| `LEGAL&CLINICIAN`          |
| `legal:hold`            | `LEGAL`                    |

So a patient sees their own diagnosis but not their own name. They also see
the public county and visits of every patient. The demo shows exactly what
the labels say. Only Admin holds both `LEGAL` and `CLINICIAN`, so only Admin
sees medical evidence.

| Endpoint                          | Who        | What it does                       |
|-----------------------------------|------------|------------------------------------|
| `GET /api/patients`               | every role | Every patient, visible cells only  |
| `GET /api/patients/{id}`          | every role | One patient, visible cells only    |
| `DELETE /api/patients/{id}`       | Admin      | Writes delete markers for the row  |
| `GET /api/patients/{id}/raw`      | Admin      | What the files still hold          |
| `POST /api/compact`               | Admin      | Runs a full compaction             |
| `GET /api/audit`                  | Admin      | The last 100 reads and deletes     |

The API refuses to delete `p-0009` and `p-0011`, because they are on legal
hold. A legal hold means the record must be kept, which pulls against the
right to erasure. The refusal is in the audit log too.

The audit log is in memory, so a restart clears it.

## The web page

`web/` is one React page, built with Vite and Tailwind and served by nginx on
http://localhost:5173. nginx also forwards `/api` to the API. Compose binds
the port to localhost only.

- Pick a role with the big buttons. For Patient, also pick a patient id.
- The table shows the same 20 patients. A cell that Accumulo did not return
  shows as a grey `hidden` chip. You cannot tell if that cell is secret or
  empty, because Accumulo does not say.
- Admin holds every label, so for Admin a missing cell shows as `none`.
- Admin also gets a side panel. Pick a patient id to see every stored cell
  of that row with its label, and the audit log.

The page uses 16px text or larger and high contrast. It works with the
keyboard. Each chip has a word on it, so colour is never the only signal.

To work on the page with live reload, run `make up`, then:

```
cd web
npm ci
npm run dev
```

`npm run dev` forwards `/api` to http://localhost:8080.
