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
writes 20 fake patients. Use `make down` to stop it, and `make clean` to stop
it and delete all data.

`make seed` writes the 20 patients again. So does every `make up`. This puts
back any patient that you deleted.

`make unit` runs the unit tests in Docker. You do not need Java installed.

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
`gateway` user that holds every label. The health check passes only after
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
