package gateway;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import org.apache.accumulo.core.client.AccumuloClient;
import org.apache.accumulo.core.client.BatchWriter;
import org.apache.accumulo.core.client.Scanner;
import org.apache.accumulo.core.client.admin.CompactionConfig;
import org.apache.accumulo.core.client.rfile.RFile;
import org.apache.accumulo.core.data.Key;
import org.apache.accumulo.core.data.Mutation;
import org.apache.accumulo.core.data.Range;
import org.apache.accumulo.core.data.Value;
import org.apache.accumulo.core.security.Authorizations;
import org.apache.accumulo.core.security.ColumnVisibility;
import org.apache.hadoop.conf.Configuration;
import org.apache.hadoop.fs.FileSystem;
import org.apache.hadoop.fs.Path;
import org.apache.hadoop.io.Text;

/** All the Accumulo code in the demo. One table, one service user. */
public class Store {
  static final String TABLE = "patients";

  /** One cell. {@code deleted} is true only for delete markers that {@link #raw} finds in files. */
  public record Cell(String row, String family, String qualifier, String visibility, String value,
      boolean deleted) {}

  private final AccumuloClient client;
  // NOTE: in memory, last compaction only. A restart forgets it.
  private volatile List<String> replaced = List.of();

  /** Uses the client's user for every call. That user must hold every label. */
  public Store(AccumuloClient client) {
    this.client = client;
  }

  /** Writes one cell with its label. */
  public void write(Cell c) throws Exception {
    Mutation m = new Mutation(c.row());
    m.put(c.family(), c.qualifier(), new ColumnVisibility(c.visibility()), c.value());
    try (BatchWriter w = client.createBatchWriter(TABLE)) {
      w.addMutation(m);
    }
  }

  /**
   * Returns the cells that {@code auths} can see. A null row means the whole table.
   * Always scan with the caller's auths. The service user holds every label so that it can write
   * and delete any cell. Its own full set must never decide what a caller can read.
   */
  public List<Cell> scan(String row, Authorizations auths) throws Exception {
    try (Scanner s = client.createScanner(TABLE, auths)) {
      if (row != null) {
        s.setRange(Range.exact(row));
      }
      return cells(s);
    }
  }

  /** Writes a delete marker for every cell in the row. Returns the number of cells. */
  public int delete(String row) throws Exception {
    List<Cell> cells = scan(row, allAuths());
    if (cells.isEmpty()) {
      return 0;
    }
    // Put the cells in files first, as they would be for any record older than a few minutes.
    // Otherwise the flush in raw() drops cells that were written and deleted in memory.
    client.tableOperations().flush(TABLE, null, null, true);
    Mutation m = new Mutation(row);
    for (Cell c : cells) {
      m.putDelete(c.family(), c.qualifier(), new ColumnVisibility(c.visibility()));
    }
    try (BatchWriter w = client.createBatchWriter(TABLE)) {
      w.addMutation(m);
    }
    return cells.size();
  }

  /**
   * Full major compaction. This is the step that drops deleted cells from the files in use.
   * Returns the files it replaced. They stay on HDFS until the garbage collector deletes them.
   */
  public List<String> compact() throws Exception {
    client.tableOperations().flush(TABLE, null, null, true);
    List<String> before = tableFiles();
    client.tableOperations().compact(TABLE, new CompactionConfig().setFlush(true).setWait(true));
    List<String> old = new ArrayList<>(before);
    old.removeAll(tableFiles());
    replaced = List.copyOf(old);
    return replaced;
  }

  /** One file that the last compaction replaced, and whether it is still on HDFS. */
  public record OldFile(String path, boolean exists) {}

  /** Checks the files from the last compaction. When none exist, the garbage collector is done. */
  public List<OldFile> oldFiles() throws Exception {
    List<OldFile> out = new ArrayList<>();
    for (String f : replaced) {
      Path p = new Path(f);
      out.add(new OldFile(f, p.getFileSystem(new Configuration()).exists(p)));
    }
    return out;
  }

  /**
   * Reads the row straight from the table's files, with no visibility filter and no delete
   * filter. This shows what is physically stored, including cells that a scan no longer returns.
   */
  public List<Cell> raw(String row) throws Exception {
    // Write the in-memory cells to a file first. A flush keeps deleted cells and delete markers.
    client.tableOperations().flush(TABLE, null, null, true);
    List<String> files = tableFiles();
    if (files.isEmpty()) {
      return List.of();
    }
    FileSystem fs = new Path(files.get(0)).getFileSystem(new Configuration());
    try (Scanner s = RFile.newScanner().from(files.toArray(String[]::new)).withFileSystem(fs)
        .withoutSystemIterators().build()) {
      s.setRange(Range.exact(row));
      return cells(s);
    }
  }

  /**
   * Lists the patients table's files from Accumulo's metadata table. This reads the 2.1 internal
   * layout (row "id;end" or "id<", family "file", qualifier = path), which is not public API.
   */
  private List<String> tableFiles() throws Exception {
    String id = client.tableOperations().tableIdMap().get(TABLE);
    List<String> files = new ArrayList<>();
    try (Scanner s = client.createScanner("accumulo.metadata", Authorizations.EMPTY)) {
      s.setRange(new Range(id + ";", id + "<"));
      s.fetchColumnFamily(new Text("file"));
      for (Map.Entry<Key, Value> e : s) {
        files.add(e.getKey().getColumnQualifier().toString());
      }
    }
    return files;
  }

  private Authorizations allAuths() throws Exception {
    return client.securityOperations().getUserAuthorizations(client.whoami());
  }

  private static List<Cell> cells(Iterable<Map.Entry<Key, Value>> entries) {
    List<Cell> out = new ArrayList<>();
    for (Map.Entry<Key, Value> e : entries) {
      Key k = e.getKey();
      out.add(new Cell(k.getRow().toString(), k.getColumnFamily().toString(),
          k.getColumnQualifier().toString(), k.getColumnVisibility().toString(),
          e.getValue().toString(), k.isDeleted()));
    }
    return out;
  }
}
