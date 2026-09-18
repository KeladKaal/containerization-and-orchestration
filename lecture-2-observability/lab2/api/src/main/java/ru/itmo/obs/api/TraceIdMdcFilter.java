package ru.itmo.obs.api;

import io.micrometer.tracing.Span;
import io.micrometer.tracing.Tracer;
import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import org.slf4j.MDC;
import org.springframework.core.Ordered;
import org.springframework.core.annotation.Order;
import org.springframework.stereotype.Component;
import org.springframework.web.filter.OncePerRequestFilter;

import java.io.IOException;

/**
 * Micrometer кладёт в MDC ключи traceId/spanId; Loki-запросы и Jaeger-ссылки в этой лабе
 * работают по snake_case trace_id — фильтр кладёт их под нужными именами, а не переименовывает в энкодере.
 */
@Component
@Order(Ordered.LOWEST_PRECEDENCE)
public class TraceIdMdcFilter extends OncePerRequestFilter {

    static final String TRACE_ID = "trace_id";
    static final String SPAN_ID = "span_id";

    private final Tracer tracer;

    TraceIdMdcFilter(Tracer tracer) {
        this.tracer = tracer;
    }

    @Override
    protected void doFilterInternal(HttpServletRequest request, HttpServletResponse response, FilterChain chain)
            throws ServletException, IOException {
        Span span = tracer.currentSpan();
        if (span == null) {
            chain.doFilter(request, response);
            return;
        }
        MDC.put(TRACE_ID, span.context().traceId());
        MDC.put(SPAN_ID, span.context().spanId());
        response.setHeader("X-Trace-Id", span.context().traceId());
        try {
            chain.doFilter(request, response);
        } finally {
            MDC.remove(TRACE_ID);
            MDC.remove(SPAN_ID);
        }
    }
}
