package gateway;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.io.File;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.stream.Collectors;

import org.apache.accumulo.core.client.AccumuloClient;
import org.apache.accumulo.core.client.security.tokens.PasswordToken;
import org.apache.accumulo.core.security.Authorizations;
import org.apache.accumulo.core.security.TablePermission;
import org.apache.accumulo.minicluster.MiniAccumuloCluster;
import org.apache.accumulo.minicluster.MiniAccumuloConfig;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class StoreTest {
  @TempDir
  static File dir;
  static MiniAccumuloCluster mac;
  static AccumuloClient gatewayClient;
  static Store store;

  @BeforeAll
  static void start() throws Exception {
    // A fast garbage collector, so the test can wait for it. The 10s gap keeps the old files on
    // disk for the check right after the compaction.
    mac = new MiniAccumuloCluster(new MiniAccumuloConfig(dir, "root-secret")
        .setSiteConfig(Map.of("gc.cycle.start", "1s", "gc.cycle.delay", "10s")));
    mac.start();
    try (AccumuloClient root = mac.createAccumuloClient("root", new PasswordToken("root-secret"))) {
      root.tableOperations().create(Store.TABLE);
      root.securityOperations().createLocalUser("gateway", new PasswordToken("gw"));
      root.securityOperations().changeUserAuthorizations("gateway",
          new Authorizations("PII", "CLINICIAN", "LEGAL", "PATIENT_p0001"));
      for (TablePermission p : List.of(TablePermission.READ, TablePermission.WRITE,
          TablePermission.ALTER_TABLE)) {
        root.securityOperations().grantTablePermission("gateway", Store.TABLE, p);
      }
    }
    gatewayClient = mac.createAccumuloClient("gateway", new PasswordToken("gw"));
    store = new Store(gatewayClient);
  }

  @AfterAll
  static void stop() throws Exception {
    gatewayClient.close();
    mac.stop();
  }

  @Test
  void scanUsesCallerAuthsNotTheServiceUsersAuths() throws Exception {
    writePatient("p-0001");

    // The gateway user holds every label, but a scan with no auths sees only public cells.
    assertEquals(Set.of("info:county"), columns(store.scan("p-0001", Authorizations.EMPTY)));
    assertEquals(Set.of("info:county", "info:name"),
        columns(store.scan("p-0001", new Authorizations("PII"))));
    assertEquals(Set.of("info:county", "clinical:diagnosis"),
        columns(store.scan("p-0001", new Authorizations("PATIENT_p0001"))));
    assertEquals(Set.of("info:county", "info:name", "legal:case_note"),
        columns(store.scan("p-0001", new Authorizations("PII", "LEGAL"))));
    assertEquals(Set.of("info:county", "info:name", "clinical:diagnosis", "legal:case_note",
        "legal:medical_evidence"),
        columns(store.scan("p-0001", new Authorizations("PII", "CLINICIAN", "LEGAL"))));
  }

  @Test
  void deletedCellsStayInFilesUntilCompaction() throws Exception {
    writePatient("p-0002");
    Authorizations all = new Authorizations("PII", "CLINICIAN", "LEGAL");

    assertEquals(5, store.delete("p-0002"));
    assertTrue(store.scan("p-0002", all).isEmpty(), "reads return nothing after delete");

    List<Store.Cell> raw = store.raw("p-0002");
    assertEquals(5, raw.stream().filter(c -> !c.deleted()).count(), "old cells still on disk");
    assertEquals(5, raw.stream().filter(Store.Cell::deleted).count(), "plus delete markers");
    assertTrue(raw.stream().anyMatch(c -> c.value().equals("Test Person")));

    List<String> old = store.compact();
    assertTrue(store.raw("p-0002").isEmpty(), "compaction drops it from the files in use");
    assertFalse(old.isEmpty(), "compaction replaced some files");
    assertTrue(store.oldFiles().stream().allMatch(Store.OldFile::exists),
        "the old files are still on disk right after the compaction");

    long deadline = System.currentTimeMillis() + 60_000;
    while (store.oldFiles().stream().anyMatch(Store.OldFile::exists)) {
      assertTrue(System.currentTimeMillis() < deadline, "garbage collector removed the old files");
      Thread.sleep(500);
    }
  }

  private static void writePatient(String row) throws Exception {
    store.write(new Store.Cell(row, "info", "name", "PII", "Test Person", false));
    store.write(new Store.Cell(row, "info", "county", "", "Dublin", false));
    store.write(new Store.Cell(row, "clinical", "diagnosis", "CLINICIAN|PATIENT_p0001", "asthma",
        false));
    store.write(new Store.Cell(row, "legal", "case_note", "LEGAL", "note", false));
    store.write(new Store.Cell(row, "legal", "medical_evidence", "LEGAL&CLINICIAN", "letter",
        false));
  }

  private static Set<String> columns(List<Store.Cell> cells) {
    return cells.stream().map(c -> c.family() + ":" + c.qualifier()).collect(Collectors.toSet());
  }
}
