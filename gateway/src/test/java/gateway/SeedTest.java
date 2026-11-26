package gateway;

import static org.junit.jupiter.api.Assertions.assertEquals;

import java.util.List;
import java.util.Set;
import java.util.stream.Collectors;
import org.junit.jupiter.api.Test;

class SeedTest {
  @Test
  void generatesTwentyPatientsWithKyleFirstAndTwoHolds() {
    List<Store.Cell> cells = Seed.generate();

    assertEquals(Seed.PATIENTS, cells.stream().map(Store.Cell::row).distinct().count());
    assertEquals("Kyle Byrne", value(cells, "p-0001", "name"));
    assertEquals("false", value(cells, "p-0001", "hold"));

    Set<String> held = cells.stream()
        .filter(c -> c.qualifier().equals("hold") && c.value().equals("true"))
        .map(Store.Cell::row).collect(Collectors.toSet());
    assertEquals(2, held.size());
    assertEquals(cells, Seed.generate(), "same data on every run");
  }

  private static String value(List<Store.Cell> cells, String row, String qualifier) {
    return cells.stream().filter(c -> c.row().equals(row) && c.qualifier().equals(qualifier))
        .findFirst().orElseThrow().value();
  }
}
