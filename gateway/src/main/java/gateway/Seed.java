package gateway;

import com.google.gson.Gson;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.util.ArrayList;
import java.util.List;
import java.util.Random;

/**
 * Writes 20 made-up patients through the gateway's /write endpoint. Every value is generated.
 * The fixed random seed gives the same data on every run. Usage: Seed http://gateway:8081
 */
public class Seed {
  static final int PATIENTS = 20; // init.sh gives the gateway PATIENT_p0001 to PATIENT_p0020.

  static final String[] FIRST = {"Aoife", "Ciara", "Niamh", "Orla", "Sinead", "Roisin", "Eimear",
      "Grainne", "Maeve", "Clodagh", "Sean", "Ciaran", "Darragh", "Eoin", "Cathal", "Fionn",
      "Padraig", "Declan", "Brendan", "Tadhg"};
  static final String[] LAST = {"Murphy", "Kelly", "O'Sullivan", "Walsh", "O'Brien", "Ryan",
      "Doyle", "McCarthy", "Gallagher", "Doherty", "Kennedy", "Lynch", "Quinn", "Carroll", "Daly"};
  static final String[] COUNTIES = {"Carlow", "Cavan", "Clare", "Cork", "Donegal", "Dublin",
      "Galway", "Kerry", "Kildare", "Kilkenny", "Laois", "Leitrim", "Limerick", "Longford", "Louth",
      "Mayo", "Meath", "Monaghan", "Offaly", "Roscommon", "Sligo", "Tipperary", "Waterford",
      "Westmeath", "Wexford", "Wicklow"};
  static final String[] DIAGNOSES = {"asthma", "type 2 diabetes", "high blood pressure",
      "migraine", "eczema", "coeliac disease", "anxiety", "back pain", "hay fever"};
  static final String[] NOTES = {"Reviewed medication. Review again in 3 months.",
      "Referred to a specialist.", "Stable. No change to treatment.", "Blood tests ordered.",
      "Advised on diet and exercise."};
  static final String[] CASE_NOTES = {"Complaint about a delayed referral. File open.",
      "Insurance claim after a fall at work.", "Records requested by a solicitor."};
  static final String[] EVIDENCE = {"GP letter on file.", "Specialist report attached.",
      "X-ray results sent to the solicitor."};

  /** Writes the patients through the gateway at args[0], then compacts. */
  public static void main(String[] args) throws Exception {
    String base = args.length > 0 ? args[0] : "http://localhost:8081";
    HttpClient http = HttpClient.newHttpClient();
    Gson gson = new Gson();
    List<Store.Cell> cells = generate();
    for (Store.Cell c : cells) {
      HttpRequest req = HttpRequest.newBuilder(URI.create(base + "/write"))
          .header("Content-Type", "application/json")
          .POST(HttpRequest.BodyPublishers.ofString(gson.toJson(c))).build();
      HttpResponse<String> res = http.send(req, HttpResponse.BodyHandlers.ofString());
      if (res.statusCode() != 200) {
        throw new IllegalStateException("write failed: " + res.statusCode() + " " + res.body());
      }
    }
    // Compact, so the files hold one version of each cell even after many seeds. Then /raw
    // shows only what the delete flow adds.
    HttpResponse<String> res = http.send(HttpRequest.newBuilder(URI.create(base + "/compact"))
        .POST(HttpRequest.BodyPublishers.noBody()).build(), HttpResponse.BodyHandlers.ofString());
    if (res.statusCode() != 200) {
      throw new IllegalStateException("compact failed: " + res.statusCode() + " " + res.body());
    }
    System.out.println("seeded " + PATIENTS + " patients, " + cells.size() + " cells");
  }

  static List<Store.Cell> generate() {
    Random rnd = new Random(42);
    // Two patients on legal hold, never p-0001.
    int hold1 = 2 + rnd.nextInt(PATIENTS - 1);
    int hold2;
    do {
      hold2 = 2 + rnd.nextInt(PATIENTS - 1);
    } while (hold2 == hold1);

    List<Store.Cell> cells = new ArrayList<>();
    for (int i = 1; i <= PATIENTS; i++) {
      String row = String.format("p-%04d", i);
      String self = String.format("PATIENT_p%04d", i);
      boolean first = i == 1;
      final boolean hold = i == hold1 || i == hold2;

      String name = first ? "Kyle Byrne" : pick(rnd, FIRST) + " " + pick(rnd, LAST);
      String county = first ? "Dublin" : pick(rnd, COUNTIES);
      String visits = first ? "4" : String.valueOf(1 + rnd.nextInt(12));
      String diagnosis = first ? "asthma" : pick(rnd, DIAGNOSES);
      String notes = pick(rnd, NOTES);

      cells.add(cell(row, "info", "name", "PII", name));
      cells.add(cell(row, "info", "county", "", county));
      cells.add(cell(row, "info", "visits", "", visits));
      cells.add(cell(row, "clinical", "diagnosis", "CLINICIAN|" + self, diagnosis));
      cells.add(cell(row, "clinical", "notes", "CLINICIAN", notes));
      if (first || hold || rnd.nextInt(4) == 0) {
        cells.add(cell(row, "legal", "case_note", "LEGAL", pick(rnd, CASE_NOTES)));
        // Needs both labels, so only someone who is Legal and Clinician (Admin here) sees it.
        cells.add(cell(row, "legal", "medical_evidence", "LEGAL&CLINICIAN", pick(rnd, EVIDENCE)));
      }
      cells.add(cell(row, "legal", "hold", "LEGAL", String.valueOf(hold)));
    }
    return cells;
  }

  private static Store.Cell cell(String row, String fam, String qual, String vis, String value) {
    return new Store.Cell(row, fam, qual, vis, value, false);
  }

  private static String pick(Random rnd, String[] options) {
    return options[rnd.nextInt(options.length)];
  }
}
