package com.hexglobe;

import android.app.Activity;
import android.os.Build;
import android.os.Bundle;
import android.view.View;
import android.view.Window;
import android.view.WindowInsets;
import android.view.WindowInsetsController;
import android.view.WindowManager;

import com.hexglobe.mobile.EbitenView;
import com.hexglobe.mobile.Mobile;

import go.Seq;

public class MainActivity extends Activity {
    private EbitenView ebitenView;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        Seq.setContext(getApplicationContext());
        Mobile.dummy();
        Mobile.setSaveDir(getFilesDir().getAbsolutePath());
        configureImmersiveWindow();
        setContentView(R.layout.activity_main);
        ebitenView = findViewById(R.id.ebitenview);
        applyImmersiveMode();
    }

    @Override
    protected void onPause() {
        super.onPause();
        if (ebitenView != null) {
            ebitenView.suspendGame();
        }
    }

    @Override
    protected void onResume() {
        super.onResume();
        if (ebitenView != null) {
            ebitenView.resumeGame();
        }
        applyImmersiveMode();
    }

    @Override
    public void onWindowFocusChanged(boolean hasFocus) {
        super.onWindowFocusChanged(hasFocus);
        if (hasFocus) {
            applyImmersiveMode();
        }
    }

    private void configureImmersiveWindow() {
        Window window = getWindow();
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            WindowManager.LayoutParams attrs = window.getAttributes();
            attrs.layoutInDisplayCutoutMode = WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES;
            window.setAttributes(attrs);
        }
    }

    private void applyImmersiveMode() {
        Window window = getWindow();
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            WindowInsetsController controller = window.getInsetsController();
            if (controller != null) {
                controller.hide(WindowInsets.Type.statusBars() | WindowInsets.Type.navigationBars());
                controller.setSystemBarsBehavior(WindowInsetsController.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE);
            }
            return;
        }

        View decor = window.getDecorView();
        decor.setSystemUiVisibility(
            View.SYSTEM_UI_FLAG_LAYOUT_STABLE
                | View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION
                | View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN
                | View.SYSTEM_UI_FLAG_HIDE_NAVIGATION
                | View.SYSTEM_UI_FLAG_FULLSCREEN
                | View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY
        );
    }
}
