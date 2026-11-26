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

This builds the images, starts the stack and waits until Accumulo is ready.
Use `make down` to stop it, and `make clean` to stop it and delete all data.

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
that step.
