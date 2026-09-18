package ru.itmo.obs.api;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;
import org.springframework.web.client.RestClient;

import java.util.Map;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.ThreadLocalRandom;
import java.util.concurrent.atomic.AtomicInteger;

/**
 * Стреляет по самому себе через Service DNS, чтобы всплеск RPS был виден на RED-дашборде
 * и распределялся по всем репликам, а не только по той, что приняла /load.
 */
@Component
public class LoadGenerator {

    private static final Logger log = LoggerFactory.getLogger(LoadGenerator.class);

    private static final int MAX_REQUESTS = 20_000;
    private static final int MAX_CONCURRENCY = 100;

    private final RestClient client;
    private final AtomicInteger burstSeq = new AtomicInteger();

    LoadGenerator(RestClient.Builder builder, @Value("${app.self-base-url}") String selfBaseUrl) {
        this.client = builder.baseUrl(selfBaseUrl).build();
        log.info("load generator targets {}", selfBaseUrl);
    }

    Map<String, Object> fire(int requests, int concurrency, double failRatio, double slowRatio) {
        int total = Math.min(Math.max(requests, 1), MAX_REQUESTS);
        int threads = Math.min(Math.max(concurrency, 1), MAX_CONCURRENCY);
        double fail = clampRatio(failRatio);
        double slow = clampRatio(slowRatio);
        int burstId = burstSeq.incrementAndGet();

        ExecutorService pool = Executors.newFixedThreadPool(threads);
        for (int i = 0; i < total; i++) {
            pool.submit(() -> call(pickPath(fail, slow)));
        }
        // очередь уже заполнена: пул доработает её и закроется сам, ответ отдаём сразу
        pool.shutdown();

        log.info("load burst {} scheduled: {} requests, {} threads, fail={}, slow={}",
                burstId, total, threads, fail, slow);

        return Map.of(
                "burst", burstId,
                "requests", total,
                "concurrency", threads,
                "fail_ratio", fail,
                "slow_ratio", slow,
                "status", "scheduled");
    }

    private String pickPath(double failRatio, double slowRatio) {
        double dice = ThreadLocalRandom.current().nextDouble();
        if (dice < failRatio) {
            return "/fail";
        }
        if (dice < failRatio + slowRatio) {
            return "/slow?ms=" + ThreadLocalRandom.current().nextInt(1_000, 3_001);
        }
        return "/health";
    }

    private void call(String path) {
        try {
            client.get()
                    .uri(path)
                    .retrieve()
                    .onStatus(status -> true, (request, response) -> { /* 5xx здесь ожидаем, это часть сценария */ })
                    .toBodilessEntity();
        } catch (RuntimeException e) {
            log.warn("load request to {} failed: {}", path, e.toString());
        }
    }

    private static double clampRatio(double value) {
        return Math.min(Math.max(value, 0.0), 1.0);
    }
}
