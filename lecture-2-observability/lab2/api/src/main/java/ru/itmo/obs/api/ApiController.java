package ru.itmo.obs.api;

import io.micrometer.core.instrument.Counter;
import io.micrometer.core.instrument.MeterRegistry;
import io.micrometer.tracing.Span;
import io.micrometer.tracing.Tracer;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.server.ResponseStatusException;

import java.util.Map;
import java.util.concurrent.ThreadLocalRandom;

@RestController
public class ApiController {

    private static final Logger log = LoggerFactory.getLogger(ApiController.class);

    private final Tracer tracer;
    private final LoadGenerator loadGenerator;
    private final Counter errorCounter;

    ApiController(Tracer tracer, LoadGenerator loadGenerator, MeterRegistry registry) {
        this.tracer = tracer;
        this.loadGenerator = loadGenerator;
        this.errorCounter = Counter.builder("app.errors")
                .description("Synthetic errors produced by /fail")
                .tag("endpoint", "/fail")
                .register(registry);
    }

    @GetMapping(value = "/health", produces = MediaType.TEXT_PLAIN_VALUE)
    public String health() {
        return "ok";
    }

    @GetMapping("/fail")
    public void fail() {
        errorCounter.increment();
        IllegalStateException cause = new IllegalStateException("synthetic failure from /fail");

        Span span = tracer.currentSpan();
        if (span != null) {
            span.error(cause);
            span.tag("app.fault", "synthetic");
        }

        log.error("request failed on purpose", cause);
        throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, "synthetic failure", cause);
    }

    @GetMapping("/slow")
    public Map<String, Object> slow(@RequestParam(required = false) Integer ms) throws InterruptedException {
        int sleepMs = ms != null ? Math.min(Math.max(ms, 0), 30_000)
                : ThreadLocalRandom.current().nextInt(1_000, 3_001);

        Span child = tracer.nextSpan().name("slow-op").start();
        try (Tracer.SpanInScope ignored = tracer.withSpan(child)) {
            child.tag("sleep.ms", String.valueOf(sleepMs));
            Thread.sleep(sleepMs);
        } finally {
            child.end();
        }

        log.info("slow endpoint slept for {} ms", sleepMs);
        return Map.of("slept_ms", sleepMs);
    }

    @GetMapping("/load")
    public Map<String, Object> load(@RequestParam(defaultValue = "300") int requests,
                                    @RequestParam(defaultValue = "20") int concurrency,
                                    @RequestParam(defaultValue = "0.2") double fail,
                                    @RequestParam(defaultValue = "0.1") double slow) {
        return loadGenerator.fire(requests, concurrency, fail, slow);
    }
}
