package dev.oneo.nsgo;

import android.app.Activity;
import android.content.Context;
import android.content.Intent;
import android.content.MutableContextWrapper;
import android.os.Handler;
import android.os.Looper;
import android.util.Log;
import android.view.ViewGroup;
import java.util.HashMap;
import java.util.Map;
import java.util.TimeZone;

/** One Go engine and one core.Video, retained by the playback service. */
final class PlaybackRuntime {
    static NsgoView view;
    static Map<String, Object> state = new HashMap<>();
    private static MutableContextWrapper context;
    private static Activity activity;
    private static final Handler ui = new Handler(Looper.getMainLooper());

    static NsgoView attach(Activity host) {
        activity = host;
        if (view == null) {
            context = new MutableContextWrapper(host);
            view = new NsgoView(context);
            view.setParams(Map.of("woodwind.android", true, "woodwind.timeZone", TimeZone.getDefault().getID()));
            view.handle("playback.update", (data, reply) -> {
                if (data instanceof Map) update((Map<String, Object>) data);
                reply.resolve(null);
            });
            view.setErrorListener(message -> {
                Log.e("Woodwind", message);
                command("pause", 0);
            });
        } else {
            context.setBaseContext(host);
            if (view.getParent() instanceof ViewGroup) ((ViewGroup) view.getParent()).removeView(view);
        }
        return view;
    }

    static void detach(Activity host) {
        if (activity != host) return;
        lifecycle();
        if (view != null && view.getParent() instanceof ViewGroup) ((ViewGroup) view.getParent()).removeView(view);
        activity = null;
        context.setBaseContext(host.getApplicationContext());
        releaseIfIdle();
    }

    static void update(Map<String, Object> next) {
        // Artwork is sent only when changed; position updates retain it.
        if (!next.containsKey("artwork") && state.containsKey("artwork")) next.put("artwork", state.get("artwork"));
        state = new HashMap<>(next);
        if (!flag("hasTrack")) {
            if (PlaybackService.instance != null) PlaybackService.instance.stopSelf();
            return;
        }
        if (PlaybackService.instance != null) {
            PlaybackService.instance.publish();
        } else if (flag("playing") && context != null) {
            try {
                context.startForegroundService(new Intent(context, PlaybackService.class));
            } catch (RuntimeException e) {
                Log.e("Woodwind", "Could not start playback service", e);
                command("pause", 0);
            }
        }
    }

    static void command(String command, long position) {
        if (view != null) view.send("android.media.command", Map.of("command", command, "position", position));
    }

    static void lifecycle() {
        if (view != null) view.send("android.lifecycle", Map.of("background", true, "timeZone", TimeZone.getDefault().getID()));
    }

    static void releaseIfIdle() {
        ui.postDelayed(() -> {
            if (activity == null && PlaybackService.instance == null && view != null) {
                view.stop();
                view = null;
                context = null;
                state = new HashMap<>();
            }
        }, 500);
    }

    static boolean flag(String key) { return Boolean.TRUE.equals(state.get(key)); }
    static long number(String key) { Object n = state.get(key); return n instanceof Number ? ((Number) n).longValue() : 0; }
    static String text(String key) { Object s = state.get(key); return s instanceof String ? (String) s : ""; }
}
