package gateway;

import com.google.gson.Gson;
import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.io.InputStreamReader;
import java.net.InetSocketAddress;
import java.net.URLDecoder;
import java.nio.charset.StandardCharsets;
import java.util.Arrays;
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.Executors;
import org.apache.accumulo.core.client.Accumulo;
import org.apache.accumulo.core.client.AccumuloClient;
import org.apache.accumulo.core.security.Authorizations;

/**
 * Internal HTTP front for {@link Store}. Only the API calls it, and it has no access control of
 * its own: the API decides which auths to send. Not exposed outside the Compose network.
 */
public class Gateway {
  private static final Gson GSON = new Gson();

  record RowRequest(String row) {}

  interface Handler {
    Object handle(HttpExchange ex, Map<String, String> query) throws Exception;
  }

  /** Connects to Accumulo with the ACCUMULO_* variables and serves HTTP on port 8081. */
  public static void main(String[] args) throws IOException {
    AccumuloClient client = Accumulo.newClient()
        .to(env("ACCUMULO_INSTANCE"), env("ACCUMULO_ZOOKEEPERS"))
        .as(env("ACCUMULO_USER"), env("ACCUMULO_PASSWORD")).build();
    Store store = new Store(client);

    HttpServer server = HttpServer.create(new InetSocketAddress(8081), 0);
    route(server, "POST", "/write", (ex, q) -> {
      Store.Cell c = body(ex, Store.Cell.class);
      require(c.row(), "row");
      require(c.family(), "family");
      require(c.qualifier(), "qualifier");
      require(c.value(), "value");
      store.write(new Store.Cell(c.row(), c.family(), c.qualifier(),
          c.visibility() == null ? "" : c.visibility(), c.value(), false));
      return Map.of("ok", true);
    });
    route(server, "GET", "/scan", (ex, q) -> store.scan(q.get("row"), auths(q.get("auths"))));
    route(server, "POST", "/delete", (ex, q) -> {
      String row = require(body(ex, RowRequest.class).row(), "row");
      return Map.of("deleted", store.delete(row));
    });
    route(server, "POST", "/compact", (ex, q) -> Map.of("oldFiles", store.compact().size()));
    route(server, "GET", "/old-files", (ex, q) -> store.oldFiles());
    route(server, "GET", "/raw", (ex, q) -> store.raw(require(q.get("row"), "row")));

    server.setExecutor(Executors.newFixedThreadPool(8));
    server.start();
    System.out.println("gateway listening on :8081");
  }

  private static void route(HttpServer server, String method, String path, Handler h) {
    server.createContext(path, ex -> {
      int status = 200;
      Object out;
      try {
        if (!ex.getRequestMethod().equals(method)) {
          status = 405;
          out = Map.of("error", method + " only");
        } else {
          out = h.handle(ex, query(ex));
        }
      } catch (IllegalArgumentException e) {
        status = 400;
        out = Map.of("error", String.valueOf(e.getMessage()));
      } catch (Exception e) {
        e.printStackTrace();
        status = 500;
        out = Map.of("error", e.toString());
      }
      byte[] bytes = GSON.toJson(out).getBytes(StandardCharsets.UTF_8);
      ex.getResponseHeaders().set("Content-Type", "application/json");
      ex.sendResponseHeaders(status, bytes.length);
      try (var os = ex.getResponseBody()) {
        os.write(bytes);
      }
    });
  }

  private static <T> T body(HttpExchange ex, Class<T> type) throws IOException {
    try (var r = new InputStreamReader(ex.getRequestBody(), StandardCharsets.UTF_8)) {
      T t = GSON.fromJson(r, type);
      if (t == null) {
        throw new IllegalArgumentException("empty body");
      }
      return t;
    } catch (com.google.gson.JsonParseException e) {
      throw new IllegalArgumentException("bad JSON: " + e.getMessage());
    }
  }

  private static Map<String, String> query(HttpExchange ex) {
    Map<String, String> q = new HashMap<>();
    String raw = ex.getRequestURI().getRawQuery();
    if (raw != null) {
      for (String pair : raw.split("&")) {
        String[] kv = pair.split("=", 2);
        q.put(URLDecoder.decode(kv[0], StandardCharsets.UTF_8),
            kv.length > 1 ? URLDecoder.decode(kv[1], StandardCharsets.UTF_8) : "");
      }
    }
    return q;
  }

  /** "PII,CLINICIAN" to Authorizations. A missing or empty list means no labels at all. */
  static Authorizations auths(String csv) {
    if (csv == null || csv.isBlank()) {
      return Authorizations.EMPTY;
    }
    return new Authorizations(Arrays.stream(csv.split(",")).map(String::trim)
        .filter(s -> !s.isEmpty()).toArray(String[]::new));
  }

  private static String require(String value, String name) {
    if (value == null || value.isEmpty()) {
      throw new IllegalArgumentException("missing " + name);
    }
    return value;
  }

  private static String env(String name) {
    String v = System.getenv(name);
    if (v == null) {
      throw new IllegalStateException("set " + name);
    }
    return v;
  }
}
