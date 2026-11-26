# One image for both the HDFS and Accumulo services.
# Built locally because there is no maintained arm64 image for Accumulo 2.1 or Hadoop.
ARG ZOOKEEPER_VERSION=3.8.8

FROM zookeeper:${ZOOKEEPER_VERSION}-jre-17 AS zookeeper

# Not 17.0.21.1: its java launcher does not expand `dir/*` classpath entries, and Hadoop needs that.
FROM eclipse-temurin:17.0.21_9-jre-jammy

ARG ZOOKEEPER_VERSION
ARG HADOOP_VERSION=3.3.6
ARG ACCUMULO_VERSION=2.1.6

RUN apt-get update \
 && apt-get install -y --no-install-recommends curl procps \
 && rm -rf /var/lib/apt/lists/*

# Accumulo only needs the ZooKeeper client jars, so copy them from the official image.
COPY --from=zookeeper /apache-zookeeper-${ZOOKEEPER_VERSION}-bin /opt/zookeeper

COPY fetch.sh /usr/local/bin/fetch.sh
RUN fetch.sh hadoop/common/hadoop-${HADOOP_VERSION}/hadoop-${HADOOP_VERSION}.tar.gz /opt/hadoop \
 && rm -rf /opt/hadoop/share/doc /opt/hadoop/share/hadoop/tools
RUN fetch.sh accumulo/${ACCUMULO_VERSION}/accumulo-${ACCUMULO_VERSION}-bin.tar.gz /opt/accumulo

ENV JAVA_HOME=/opt/java/openjdk \
    ZOOKEEPER_HOME=/opt/zookeeper \
    HADOOP_HOME=/opt/hadoop \
    HADOOP_CONF_DIR=/opt/hadoop/etc/hadoop \
    HADOOP_HEAPSIZE_MAX=512m \
    ACCUMULO_HOME=/opt/accumulo \
    PATH=/opt/accumulo/bin:/opt/hadoop/bin:$PATH

COPY conf/hadoop/ /opt/hadoop/etc/hadoop/
COPY conf/accumulo/ /opt/accumulo/conf/
COPY start-hdfs.sh start-accumulo.sh init.sh /usr/local/bin/

RUN useradd --create-home demo \
 && mkdir -p /data /opt/accumulo/logs /opt/hadoop/logs \
 && chown -R demo /data /opt/accumulo/logs /opt/hadoop/logs
USER demo
