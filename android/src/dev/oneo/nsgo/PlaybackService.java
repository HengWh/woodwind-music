package dev.oneo.nsgo;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.content.pm.ServiceInfo;
import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.media.AudioAttributes;
import android.media.AudioFocusRequest;
import android.media.AudioManager;
import android.media.MediaMetadata;
import android.media.session.MediaSession;
import android.media.session.PlaybackState;
import android.os.Build;
import android.os.Handler;
import android.os.IBinder;
import android.os.Looper;
import android.os.PowerManager;
import android.os.SystemClock;
import android.util.Base64;
import android.util.Log;

/** Keeps the NSGo runtime/core.Video alive and exposes its Go queue to Android. */
public final class PlaybackService extends Service {
    static PlaybackService instance;
    private static final String CHANNEL = "woodwind.playback";
    private static final int NOTIFICATION = 1001;
    private final Handler ui = new Handler(Looper.getMainLooper());
    private MediaSession session;
    private AudioManager audio;
    private AudioFocusRequest focus;
    private PowerManager.WakeLock wake;
    private boolean focused, interrupted, foreground, receiverRegistered;
    private String artworkData = "";
    private Bitmap artwork;
    private final Runnable idle = this::stopSelf;
    private final BroadcastReceiver noisy = new BroadcastReceiver() {
        @Override public void onReceive(Context c, Intent i) { PlaybackRuntime.command("pause", 0); }
    };

    @Override public void onCreate() {
        super.onCreate();
        instance = this;
        NotificationManager notifications = getSystemService(NotificationManager.class);
        notifications.createNotificationChannel(new NotificationChannel(CHANNEL, "音乐播放", NotificationManager.IMPORTANCE_LOW));
        audio = getSystemService(AudioManager.class);
        focus = new AudioFocusRequest.Builder(AudioManager.AUDIOFOCUS_GAIN)
            .setAudioAttributes(new AudioAttributes.Builder().setUsage(AudioAttributes.USAGE_MEDIA)
                .setContentType(AudioAttributes.CONTENT_TYPE_MUSIC).build())
            .setWillPauseWhenDucked(true).setOnAudioFocusChangeListener(this::focusChanged, ui).build();
        wake = getSystemService(PowerManager.class).newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, getPackageName()+":playback");
        wake.setReferenceCounted(false);
        session = new MediaSession(this, "Woodwind Music");
        session.setFlags(MediaSession.FLAG_HANDLES_MEDIA_BUTTONS | MediaSession.FLAG_HANDLES_TRANSPORT_CONTROLS);
        session.setSessionActivity(openApp());
        session.setCallback(new MediaSession.Callback() {
            @Override public void onPlay() { PlaybackRuntime.command("play", 0); }
            @Override public void onPause() { interrupted = false; PlaybackRuntime.command("pause", 0); }
            @Override public void onSkipToNext() { PlaybackRuntime.command("next", 0); }
            @Override public void onSkipToPrevious() { PlaybackRuntime.command("previous", 0); }
            @Override public void onSeekTo(long position) { PlaybackRuntime.command("seek", position); }
            @Override public void onStop() { stopPlayback(); }
        }, ui);
        session.setActive(true);
        IntentFilter filter = new IntentFilter(AudioManager.ACTION_AUDIO_BECOMING_NOISY);
        if (Build.VERSION.SDK_INT >= 33) registerReceiver(noisy, filter, Context.RECEIVER_NOT_EXPORTED);
        else registerReceiver(noisy, filter);
        receiverRegistered = true;
    }

    @Override public int onStartCommand(Intent intent, int flags, int id) {
        // A notification is required immediately, including while the file loads.
        publish();
        String action = intent == null ? null : intent.getAction();
        if ("stop".equals(action)) stopPlayback();
        else if (action != null) { interrupted = false; PlaybackRuntime.command(action, 0); }
        return START_NOT_STICKY;
    }

    void publish() {
        if (!PlaybackRuntime.flag("hasTrack")) { stopSelf(); return; }
        boolean playing = PlaybackRuntime.flag("playing");
        boolean loading = PlaybackRuntime.flag("loading");
        String image = PlaybackRuntime.text("artwork");
        if (!image.equals(artworkData)) {
            artworkData = image;
            artwork = null;
            try {
                if (!image.isEmpty()) {
                    byte[] bytes = Base64.decode(image, Base64.DEFAULT);
                    BitmapFactory.Options options = new BitmapFactory.Options();
                    options.inSampleSize = 2;
                    artwork = BitmapFactory.decodeByteArray(bytes, 0, bytes.length, options);
                }
            } catch (RuntimeException e) { Log.w("Woodwind", "Invalid cover image", e); }
        }
        MediaMetadata.Builder metadata = new MediaMetadata.Builder()
            .putString(MediaMetadata.METADATA_KEY_TITLE, PlaybackRuntime.text("title"))
            .putString(MediaMetadata.METADATA_KEY_ARTIST, PlaybackRuntime.text("artist"))
            .putString(MediaMetadata.METADATA_KEY_ALBUM, PlaybackRuntime.text("album"))
            .putLong(MediaMetadata.METADATA_KEY_DURATION, PlaybackRuntime.number("duration"));
        if (artwork != null) metadata.putBitmap(MediaMetadata.METADATA_KEY_ALBUM_ART, artwork);
        session.setMetadata(metadata.build());
        int status = playing ? (loading ? PlaybackState.STATE_BUFFERING : PlaybackState.STATE_PLAYING) : PlaybackState.STATE_PAUSED;
        if (!PlaybackRuntime.text("error").isEmpty()) status = PlaybackState.STATE_ERROR;
        long actions = PlaybackState.ACTION_PLAY | PlaybackState.ACTION_PAUSE | PlaybackState.ACTION_PLAY_PAUSE
            | PlaybackState.ACTION_SKIP_TO_NEXT | PlaybackState.ACTION_SKIP_TO_PREVIOUS | PlaybackState.ACTION_SEEK_TO | PlaybackState.ACTION_STOP;
        PlaybackState.Builder state = new PlaybackState.Builder().setActions(actions)
            .setState(status, PlaybackRuntime.number("position"), playing && !loading ? 1 : 0, SystemClock.elapsedRealtime());
        if (status == PlaybackState.STATE_ERROR) state.setErrorMessage(PlaybackRuntime.text("error"));
        session.setPlaybackState(state.build());
        Notification.Builder builder = new Notification.Builder(this, CHANNEL)
            .setSmallIcon(android.R.drawable.ic_media_play).setContentTitle(PlaybackRuntime.text("title"))
            .setContentText(PlaybackRuntime.text("artist")).setSubText(PlaybackRuntime.text("album"))
            .setContentIntent(openApp()).setOnlyAlertOnce(true).setShowWhen(false).setOngoing(playing)
            .setVisibility(Notification.VISIBILITY_PUBLIC).setCategory(Notification.CATEGORY_TRANSPORT)
            .setDeleteIntent(action("stop"))
            .addAction(new Notification.Action.Builder(android.R.drawable.ic_media_previous, "上一首", action("previous")).build())
            .addAction(new Notification.Action.Builder(playing ? android.R.drawable.ic_media_pause : android.R.drawable.ic_media_play,
                playing ? "暂停" : "播放", action(playing ? "pause" : "play")).build())
            .addAction(new Notification.Action.Builder(android.R.drawable.ic_media_next, "下一首", action("next")).build())
            .addAction(new Notification.Action.Builder(android.R.drawable.ic_menu_close_clear_cancel, "停止", action("stop")).build())
            .setStyle(new Notification.MediaStyle().setMediaSession(session.getSessionToken()).setShowActionsInCompactView(0, 1, 2));
        if (artwork != null) builder.setLargeIcon(artwork);
        if (Build.VERSION.SDK_INT >= 29) startForeground(NOTIFICATION, builder.build(), ServiceInfo.FOREGROUND_SERVICE_TYPE_MEDIA_PLAYBACK);
        else startForeground(NOTIFICATION, builder.build());
        foreground = true;
        ui.removeCallbacks(idle);
        if (playing) {
            if (!wake.isHeld()) wake.acquire();
            if (!focused && !interrupted) {
                focused = audio.requestAudioFocus(focus) == AudioManager.AUDIOFOCUS_REQUEST_GRANTED;
                if (!focused) PlaybackRuntime.command("pause", 0);
            }
        } else {
            if (wake.isHeld()) wake.release();
            if (!interrupted) abandonFocus();
            ui.postDelayed(idle, 5*60*1000);
        }
    }

    private void focusChanged(int change) {
        if (change == AudioManager.AUDIOFOCUS_GAIN) {
            focused = true;
            if (interrupted) { interrupted = false; PlaybackRuntime.command("focus-resume", 0); }
        } else if (change == AudioManager.AUDIOFOCUS_LOSS) {
            focused = false;
            interrupted = false;
            PlaybackRuntime.command("pause", 0);
        } else {
            focused = false;
            interrupted = true;
            PlaybackRuntime.command("focus-pause", 0);
        }
    }

    private void abandonFocus() {
        audio.abandonAudioFocusRequest(focus);
        focused = false;
    }

    private PendingIntent openApp() {
        Intent i = new Intent(this, NsgoActivity.class).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK | Intent.FLAG_ACTIVITY_SINGLE_TOP | Intent.FLAG_ACTIVITY_CLEAR_TOP);
        return PendingIntent.getActivity(this, 0, i, PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
    }

    private PendingIntent action(String name) {
        return PendingIntent.getService(this, name.hashCode(), new Intent(this, PlaybackService.class).setAction(name),
            PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
    }

    private void stopPlayback() {
        interrupted = false;
        PlaybackRuntime.command("stop", 0);
        PlaybackRuntime.lifecycle();
        ui.postDelayed(this::stopSelf, 250);
    }

    @Override public void onTaskRemoved(Intent intent) {
        PlaybackRuntime.lifecycle();
        if (!PlaybackRuntime.flag("playing")) stopSelf();
    }

    @Override public void onDestroy() {
        ui.removeCallbacksAndMessages(null);
        if (wake != null && wake.isHeld()) wake.release();
        if (audio != null && focus != null) abandonFocus();
        if (receiverRegistered) unregisterReceiver(noisy);
        if (session != null) { session.setActive(false); session.release(); }
        if (foreground) stopForeground(STOP_FOREGROUND_REMOVE);
        instance = null;
        PlaybackRuntime.releaseIfIdle();
        super.onDestroy();
    }

    @Override public IBinder onBind(Intent intent) { return null; }
}
