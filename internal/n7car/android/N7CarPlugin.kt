package dev.mygo

import android.app.Dialog
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.ServiceConnection
import android.graphics.BitmapFactory
import android.media.AudioAttributes
import android.media.AudioFocusRequest
import android.media.AudioManager
import android.media.MediaMetadata
import android.media.session.MediaSession
import android.media.session.PlaybackState
import android.net.ConnectivityManager
import android.os.Binder
import android.os.Handler
import android.os.IBinder
import android.os.IInterface
import android.os.Looper
import android.os.Build
import android.os.Parcel
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
    companion object {
        var live: N7CarPlugin? = null
        val pendingVoice = ArrayList<Pair<String, String>>()
        val voiceBroadcastMethods = mapOf(
            "com.byd.action.AUTOVOICE_PLAY_RANDOM" to "playRandom",
            "com.byd.action.AUTOVOICE_RECOMMEND" to "playRecommend",
            "com.byd.action.AUTOVOICE_PLAY_LATELY" to "playLately",
            "com.byd.action.AUTOVOICE_PLAY_FAVORITE" to "playFavorite",
            "com.byd.action.AUTOVOICE_PLAY_SONG_SHEET" to "playPlaylist",
            "com.byd.action.AUTOVOICE_SEARCH_PLUS" to "searchMusic",
            "com.byd.action.AUTOVOICE_PLAY_MODE" to "setPlayPattern"
        )
        private const val AIDL_DESC = "com.bydsdk.autovoice.IBydMediaAIDL"
        private const val LISTENER_DESC = "com.bydsdk.autovoice.IBydMediaListener"
        private const val CALLBACK_DESC = "com.bydsdk.autovoice.IBydMediaCallback"
    }
    private var media: MediaSession? = null
    private var autoVoiceCallback: IBinder? = null
    private var autoVoiceBound = false
    private val mainHandler = Handler(Looper.getMainLooper())
    private val autoVoiceListener = object : Binder() {
        init {
            val binderRef = this
            attachInterface(object : IInterface {
                override fun asBinder(): IBinder = binderRef
            }, LISTENER_DESC)
        }
        override fun onTransact(code: Int, data: Parcel, reply: Parcel?, flags: Int): Boolean {
            return when (code) {
                INTERFACE_TRANSACTION -> {
                    reply?.writeString(LISTENER_DESC)
                    true
                }
                1 -> {
                    data.enforceInterface(LISTENER_DESC)
                    autoVoiceCallback = data.readStrongBinder()
                    reply?.writeNoException()
                    true
                }
                2 -> {
                    data.enforceInterface(LISTENER_DESC)
                    val method = data.readString().orEmpty()
                    val json = data.readString().orEmpty()
                    dispatchAutoVoice(method, json, acknowledge = false)
                    reply?.writeNoException()
                    true
                }
                3 -> {
                    data.enforceInterface(LISTENER_DESC)
                    val method = data.readString().orEmpty()
                    val json = data.readString().orEmpty()
                    dispatchAutoVoice(method, json, acknowledge = true)
                    reply?.writeNoException()
                    true
                }
                4 -> {
                    data.enforceInterface(LISTENER_DESC)
                    data.readString()
                    data.readString()
                    reply?.writeNoException()
                    true
                }
                else -> super.onTransact(code, data, reply, flags)
            }
        }
    }
    private val autoVoiceConn = object : ServiceConnection {
        override fun onServiceConnected(name: ComponentName?, service: IBinder?) {
            val binder = service ?: return
            runCatching {
                val data = Parcel.obtain()
                val reply = Parcel.obtain()
                try {
                    data.writeInterfaceToken(AIDL_DESC)
                    data.writeString(PluginHost.context.packageName)
                    data.writeIntArray(intArrayOf(1001))
                    data.writeStrongBinder(autoVoiceListener)
                    binder.transact(1, data, reply, 0)
                    reply.readException()
                } finally {
                    reply.recycle()
                    data.recycle()
                }
            }
        }
        override fun onServiceDisconnected(name: ComponentName?) {
            autoVoiceBound = false
            mainHandler.postDelayed({ bindAutoVoice() }, 2000)
        }
    }
    private fun bindAutoVoice() {
        if (autoVoiceBound) return
        runCatching {
            val intent = Intent("com.byd.action.AUTOVOICE_MEDIA_REGISTER").setPackage("com.byd.autovoice")
            autoVoiceBound = PluginHost.context.bindService(intent, autoVoiceConn, Context.BIND_AUTO_CREATE)
        }
    }
    private fun replyAutoVoice(resultJson: String) {
        val cb = autoVoiceCallback ?: return
        runCatching {
            val data = Parcel.obtain()
            val reply = Parcel.obtain()
            try {
                data.writeInterfaceToken(CALLBACK_DESC)
                data.writeString(PluginHost.context.packageName)
                data.writeString(resultJson)
                cb.transact(1, data, reply, 0)
                reply.readException()
            } finally {
                reply.recycle()
                data.recycle()
            }
        }
    }
    private fun dispatchAutoVoice(method: String, json: String, acknowledge: Boolean) {
        mainHandler.post {
            when (method.lowercase()) {
                "pause", "stop" -> command("pause")
                "next", "playnext" -> command("next")
                "previous", "prev", "pre", "playprev", "playprevious" -> command("previous")
                else -> PluginHost.emit("n7car", "voice", mapOf("method" to method, "json" to json))
            }
            if (acknowledge) replyAutoVoice("""{"code":0,"message":"ok"}""")
        }
    }
    private val bornAt = android.os.SystemClock.uptimeMillis()
    fun voiceFromBroadcast(method: String, json: String) {
        // The Go side needs a moment after a cold start before it can handle voice commands.
        val wait = (bornAt + 3500 - android.os.SystemClock.uptimeMillis()).coerceAtLeast(0)
        mainHandler.postDelayed({ dispatchAutoVoice(method, json, acknowledge = false) }, wait)
    }
    init {
        live = this
        mainHandler.post { bindAutoVoice() }
        synchronized(pendingVoice) {
            val items = pendingVoice.toList()
            pendingVoice.clear()
            for ((m, j) in items) voiceFromBroadcast(m, j)
        }
    }
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

class N7VoiceReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val method = N7CarPlugin.voiceBroadcastMethods[intent.action] ?: return
        val obj = JSONObject()
        intent.extras?.let { b -> for (k in b.keySet()) runCatching { obj.put(k, b.get(k)?.toString()) } }
        android.util.Log.d("MyGoAutoVoice", "broadcast ${intent.action} extras=$obj")
        val plugin = N7CarPlugin.live
        if (plugin != null) plugin.voiceFromBroadcast(method, obj.toString())
        else synchronized(N7CarPlugin.pendingVoice) { N7CarPlugin.pendingVoice.add(method to obj.toString()) }
    }
}
