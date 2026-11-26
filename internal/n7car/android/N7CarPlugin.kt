package dev.mygo

import android.app.Dialog
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.graphics.BitmapFactory
import android.media.AudioAttributes
import android.media.AudioFocusRequest
import android.media.AudioManager
import android.media.MediaMetadata
import android.media.session.MediaSession
import android.media.session.PlaybackState
import android.net.ConnectivityManager
import android.os.Handler
import android.os.Looper
import android.os.Build
import android.util.Base64
import android.view.KeyEvent
import android.view.ViewGroup
import android.webkit.CookieManager
import android.webkit.WebChromeClient
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.LinearLayout
import android.widget.Button
import org.json.JSONObject

class N7CarPlugin : MyGoPlugin {
    companion object { var live: N7CarPlugin? = null }
    private var media: MediaSession? = null
    private val audioManager by lazy { PluginHost.context.getSystemService(Context.AUDIO_SERVICE) as AudioManager }
    private var hasFocus = false
    private var focusRequested = false
    private val focusRequest by lazy {
        AudioFocusRequest.Builder(AudioManager.AUDIOFOCUS_GAIN)
            .setAudioAttributes(AudioAttributes.Builder().setUsage(AudioAttributes.USAGE_MEDIA).setContentType(AudioAttributes.CONTENT_TYPE_MUSIC).build())
            .setOnAudioFocusChangeListener({ change ->
                if (change == AudioManager.AUDIOFOCUS_LOSS || change == AudioManager.AUDIOFOCUS_LOSS_TRANSIENT || change == AudioManager.AUDIOFOCUS_LOSS_TRANSIENT_CAN_DUCK) {
                    releaseFocus()
                    command("pause")
                }
            }, Handler(Looper.getMainLooper()))
            .build()
    }
    private fun releaseFocus() {
        if (focusRequested) audioManager.abandonAudioFocusRequest(focusRequest)
        focusRequested = false
        hasFocus = false
    }
    private fun command(name: String, position: Long = 0) {
        PluginHost.emit("n7car", "command", mapOf("name" to name, "position" to position))
    }
    private fun session(): MediaSession {
        media?.let { return it }
        val s = MediaSession(PluginHost.context, "MyGo Music")
        live = this
        val receiver = ComponentName(PluginHost.context, N7MediaButtonReceiver::class.java)
        if (Build.VERSION.SDK_INT >= 31) {
            s.setMediaButtonBroadcastReceiver(receiver)
        } else {
            val intent = Intent(Intent.ACTION_MEDIA_BUTTON).setComponent(receiver)
            s.setMediaButtonReceiver(PendingIntent.getBroadcast(PluginHost.context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE))
        }
        s.setCallback(object : MediaSession.Callback() {
            override fun onPlay() = command("play")
            override fun onPause() = command("pause")
            override fun onSkipToNext() = command("next")
            override fun onSkipToPrevious() = command("previous")
            override fun onSeekTo(pos: Long) = command("seek", pos)
            override fun onStop() = command("pause")
        })
        s.setFlags(MediaSession.FLAG_HANDLES_MEDIA_BUTTONS or MediaSession.FLAG_HANDLES_TRANSPORT_CONTROLS)
        media = s
        return s
    }
    fun mediaButton(event: KeyEvent) {
        if (event.action != KeyEvent.ACTION_DOWN || event.repeatCount != 0) return
        when (event.keyCode) {
            KeyEvent.KEYCODE_MEDIA_NEXT -> command("next")
            KeyEvent.KEYCODE_MEDIA_PREVIOUS -> command("previous")
            KeyEvent.KEYCODE_MEDIA_PLAY -> command("play")
            KeyEvent.KEYCODE_MEDIA_PAUSE, KeyEvent.KEYCODE_MEDIA_STOP -> command("pause")
            KeyEvent.KEYCODE_MEDIA_PLAY_PAUSE, KeyEvent.KEYCODE_HEADSETHOOK -> command("toggle")
        }
    }
    override fun call(method: String, args: JSONObject, reply: PluginReply) {
        when (method) {
            "media" -> {
                val s = session()
                if (!args.optBoolean("showing")) { releaseFocus(); s.isActive = false; reply.success(null); return }
                val m = MediaMetadata.Builder()
                    .putString(MediaMetadata.METADATA_KEY_TITLE, args.optString("title"))
                    .putString(MediaMetadata.METADATA_KEY_ARTIST, args.optString("artist"))
                    .putString(MediaMetadata.METADATA_KEY_ALBUM, args.optString("album"))
                    .putLong(MediaMetadata.METADATA_KEY_DURATION, args.optLong("duration"))
                val art = args.optString("artwork")
                if (art.isNotEmpty()) runCatching { val b = Base64.decode(art, Base64.DEFAULT); BitmapFactory.decodeByteArray(b, 0, b.size) }.getOrNull()?.let { m.putBitmap(MediaMetadata.METADATA_KEY_ALBUM_ART, it) }
                s.setMetadata(m.build())
                val playing = args.optBoolean("playing")
                // Keep already-granted focus when the user pauses: some car
                // systems route steering-wheel keys only to its focus owner.
                // A focus loss or an empty session still releases it above.
                if (playing && !hasFocus) {
                    hasFocus = audioManager.requestAudioFocus(focusRequest) == AudioManager.AUDIOFOCUS_REQUEST_GRANTED
                    focusRequested = hasFocus
                    if (!hasFocus) command("pause")
                }
                val actions = PlaybackState.ACTION_PLAY or PlaybackState.ACTION_PAUSE or PlaybackState.ACTION_PLAY_PAUSE or PlaybackState.ACTION_SKIP_TO_NEXT or PlaybackState.ACTION_SKIP_TO_PREVIOUS or PlaybackState.ACTION_SEEK_TO
                s.setPlaybackState(PlaybackState.Builder().setActions(actions).setState(if (playing) PlaybackState.STATE_PLAYING else PlaybackState.STATE_PAUSED, args.optLong("position"), if (playing) 1f else 0f).build())
                s.isActive = true
                reply.success(null)
            }
            "signin" -> signIn(args, reply)
            "proxy" -> systemProxy(reply)
            else -> reply.failure(PluginFailure("UnknownMethod", method))
        }
    }

    private fun systemProxy(reply: PluginReply) {
        var host = ""
        var port = 0
        var exclude: List<String> = emptyList()
        runCatching {
            val cm = PluginHost.context.getSystemService(Context.CONNECTIVITY_SERVICE) as? ConnectivityManager
            val p = cm?.defaultProxy
            if (p != null && !p.host.isNullOrBlank() && p.port > 0) {
                host = p.host.trim()
                port = p.port
                exclude = p.exclusionList?.toList() ?: emptyList()
            }
        }
        if (host.isEmpty() || port <= 0) {
            val propHost = (System.getProperty("https.proxyHost") ?: System.getProperty("http.proxyHost"))?.trim().orEmpty()
            val propPort = (System.getProperty("https.proxyPort") ?: System.getProperty("http.proxyPort"))?.trim()?.toIntOrNull() ?: 0
            if (propHost.isNotEmpty() && propPort > 0) {
                host = propHost
                port = propPort
                val nonProxy = System.getProperty("http.nonProxyHosts")?.trim().orEmpty()
                if (nonProxy.isNotEmpty()) {
                    exclude = nonProxy.split('|', ',').map { it.trim() }.filter { it.isNotEmpty() }
                }
            }
        }
        reply.success(mapOf("host" to host, "port" to port, "exclude" to exclude))
    }

    private fun signIn(args: JSONObject, reply: PluginReply) {
        val activity = PluginHost.activity ?: run { reply.failure(PluginFailure("Unavailable", "没有可显示登录页的窗口")); return }
        val url = args.optString("url")
        val cookiesOf = args.optString("cookiesOf")
        if (!url.startsWith("https://") || !cookiesOf.startsWith("https://")) { reply.failure(PluginFailure("InvalidURL", "登录地址无效")); return }
        val required = args.optJSONArray("until") ?: org.json.JSONArray()
        if (required.length() == 0) { reply.failure(PluginFailure("InvalidCookies", "登录条件无效")); return }
        val dialog = Dialog(activity, android.R.style.Theme_Material_Light_NoActionBar_Fullscreen)
        val layout = LinearLayout(activity).apply { orientation = LinearLayout.VERTICAL }
        val close = Button(activity).apply { text = "关闭登录" }
        val web = WebView(activity)
        web.settings.javaScriptEnabled = true
        web.settings.domStorageEnabled = true
        val ua = args.optString("userAgent")
        if (ua.isNotEmpty()) web.settings.userAgentString = ua
        web.webViewClient = WebViewClient()
        web.webChromeClient = WebChromeClient()
        CookieManager.getInstance().setAcceptThirdPartyCookies(web, true)
        layout.addView(close)
        layout.addView(web, LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, 0, 1f))
        dialog.setContentView(layout)
        val handler = Handler(Looper.getMainLooper())
        var done = false
        fun finish(cookie: String?) {
            if (done) return
            done = true
            handler.removeCallbacksAndMessages(null)
            web.stopLoading()
            dialog.dismiss()
            web.destroy()
            if (cookie == null) reply.failure(PluginFailure("Cancelled", "登录已关闭")) else reply.success(mapOf("cookie" to cookie))
        }
        close.setOnClickListener { finish(null) }
        dialog.setOnCancelListener { finish(null) }
        dialog.show()
        val deadline = System.currentTimeMillis() + 5 * 60 * 1000L
        val poll = object : Runnable {
            override fun run() {
                if (done) return
                val cookie = CookieManager.getInstance().getCookie(cookiesOf) ?: ""
                val keys = cookie.split(';').map { it.trim().substringBefore('=') }.toSet()
                if ((0 until required.length()).all { keys.contains(required.optString(it)) }) { finish(cookie); return }
                if (System.currentTimeMillis() >= deadline) { finish(null); return }
                handler.postDelayed(this, 500)
            }
        }
        web.loadUrl(url)
        handler.postDelayed(poll, 500)
    }
}

class N7MediaButtonReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != Intent.ACTION_MEDIA_BUTTON) return
        @Suppress("DEPRECATION")
        val event = intent.getParcelableExtra<KeyEvent>(Intent.EXTRA_KEY_EVENT) ?: return
        N7CarPlugin.live?.mediaButton(event)
    }
}
