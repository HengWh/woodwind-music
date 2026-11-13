package dev.oneo.nsgo;

import android.app.Activity;
import android.content.Intent;
import android.content.res.Configuration;
import android.os.Build;
import android.os.Bundle;
import android.view.KeyEvent;
import android.view.Window;
import android.view.WindowManager;

/** The NSGo UI attaches to the retained runtime owned by PlaybackService. */
public class NsgoActivity extends Activity {
    private NsgoView view;

    @Override public void onCreate(Bundle state) {
        int night = getResources().getConfiguration().uiMode & Configuration.UI_MODE_NIGHT_MASK;
        if (night != Configuration.UI_MODE_NIGHT_YES) setTheme(android.R.style.Theme_DeviceDefault_Light_NoActionBar);
        super.onCreate(state);
        Window w = getWindow();
        WindowManager.LayoutParams lp = w.getAttributes();
        lp.layoutInDisplayCutoutMode = WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES;
        w.setAttributes(lp);
        w.setSoftInputMode(WindowManager.LayoutParams.SOFT_INPUT_ADJUST_RESIZE);
        w.setStatusBarColor(android.graphics.Color.TRANSPARENT);
        w.setNavigationBarColor(android.graphics.Color.TRANSPARENT);
        if (Build.VERSION.SDK_INT >= 30) w.setDecorFitsSystemWindows(false);
        view = PlaybackRuntime.attach(this);
        view.setFlags(getIntent().getExtras());
        view.setTest(getIntent().getStringExtra(TestAgent.EXTRA));
        setContentView(view);
    }

    @Override protected void onStop() {
        PlaybackRuntime.lifecycle();
        super.onStop();
    }

    @Override protected void onDestroy() {
        PlaybackRuntime.detach(this);
        super.onDestroy();
    }

    @Override public void onBackPressed() { if (!view.back()) super.onBackPressed(); }

    @Override protected void onActivityResult(int request, int result, Intent data) {
        super.onActivityResult(request, result, data);
        view.activityResult(request, result, data);
    }

    @Override public void onRequestPermissionsResult(int request, String[] permissions, int[] results) {
        super.onRequestPermissionsResult(request, permissions, results);
        view.permissionResult(request, permissions, results);
    }

    @Override public boolean onKeyDown(int key, KeyEvent event) {
        String command = mediaCommand(key);
        if (command != null) {
            if (event.getRepeatCount() == 0) PlaybackRuntime.command(command, 0);
            return true;
        }
        return super.onKeyDown(key, event);
    }

    @Override public boolean onKeyUp(int key, KeyEvent event) {
        return mediaCommand(key) != null || super.onKeyUp(key, event);
    }

    private String mediaCommand(int key) {
        switch (key) {
            case KeyEvent.KEYCODE_MEDIA_PLAY: return "play";
            case KeyEvent.KEYCODE_MEDIA_PAUSE: return "pause";
            case KeyEvent.KEYCODE_MEDIA_PLAY_PAUSE:
            case KeyEvent.KEYCODE_HEADSETHOOK: return "toggle";
            case KeyEvent.KEYCODE_MEDIA_NEXT: return "next";
            case KeyEvent.KEYCODE_MEDIA_PREVIOUS: return "previous";
            case KeyEvent.KEYCODE_MEDIA_STOP: return "stop";
            default: return null;
        }
    }

    void followAppearance(boolean dark) {
        int theme = dark ? android.R.style.Theme_DeviceDefault_NoActionBar : android.R.style.Theme_DeviceDefault_Light_NoActionBar;
        setTheme(theme);
        getTheme().applyStyle(theme, true);
        android.content.res.TypedArray a = getTheme().obtainStyledAttributes(new int[]{android.R.attr.windowBackground});
        getWindow().setBackgroundDrawable(a.getDrawable(0));
        a.recycle();
    }
}
