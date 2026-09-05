"use client";

import { useEffect, useRef, useState } from "react";
import type { CSSProperties, KeyboardEvent as ReactKeyboardEvent } from "react";
import {
	ArrowClockwiseIcon,
	ArrowCounterClockwiseIcon,
	CornersOutIcon,
	PauseIcon,
	PictureInPictureIcon,
	PlayIcon,
	SpeakerHighIcon,
	SpeakerSlashIcon,
	SpinnerGapIcon,
	WarningCircleIcon,
} from "@phosphor-icons/react";

const PLAYER_PREFERENCES_KEY = "eterealink-video-preferences";
const CONTROL_HIDE_DELAY = 2_400;
const PLAYBACK_RATES = [0.5, 0.75, 1, 1.25, 1.5, 2];

type VideoPlayerProps = {
	name: string;
	url: string;
};

type PictureInPictureDocument = Document & {
	pictureInPictureElement?: Element | null;
	pictureInPictureEnabled?: boolean;
	exitPictureInPicture?: () => Promise<void>;
};

type PictureInPictureVideo = HTMLVideoElement & {
	requestPictureInPicture?: () => Promise<unknown>;
};

export function VideoPlayer({ name, url }: VideoPlayerProps) {
	const frameRef = useRef<HTMLDivElement>(null);
	const videoRef = useRef<HTMLVideoElement>(null);
	const controlTimerRef = useRef<number | null>(null);
	const videoClickTimerRef = useRef<number | null>(null);
	const playingRef = useRef(false);
	const [playing, setPlaying] = useState(false);
	const [currentTime, setCurrentTime] = useState(0);
	const [duration, setDuration] = useState(0);
	const [volume, setVolume] = useState(1);
	const [muted, setMuted] = useState(false);
	const [playbackRate, setPlaybackRate] = useState(1);
	const [resolution, setResolution] = useState("");
	const [fullscreen, setFullscreen] = useState(false);
	const [pictureInPictureAvailable, setPictureInPictureAvailable] = useState(false);
	const [controlsVisible, setControlsVisible] = useState(true);
	const [loading, setLoading] = useState(true);
	const [buffering, setBuffering] = useState(false);
	const [playbackError, setPlaybackError] = useState("");

	useEffect(() => {
		function updateFullscreen() {
			setFullscreen(document.fullscreenElement === frameRef.current);
		}
		document.addEventListener("fullscreenchange", updateFullscreen);
		return () => {
			document.removeEventListener("fullscreenchange", updateFullscreen);
			if (controlTimerRef.current !== null) window.clearTimeout(controlTimerRef.current);
			if (videoClickTimerRef.current !== null) window.clearTimeout(videoClickTimerRef.current);
		};
	}, []);

	function clearControlTimer() {
		if (controlTimerRef.current === null) return;
		window.clearTimeout(controlTimerRef.current);
		controlTimerRef.current = null;
	}

	function scheduleControlHide() {
		clearControlTimer();
		if (!playingRef.current) return;
		controlTimerRef.current = window.setTimeout(() => setControlsVisible(false), CONTROL_HIDE_DELAY);
	}

	function showControls(keepVisible = false) {
		setControlsVisible(true);
		clearControlTimer();
		if (!keepVisible) scheduleControlHide();
	}

	async function togglePlayback() {
		const video = videoRef.current;
		if (!video) return;
		try {
			if (video.paused) await video.play();
			else video.pause();
		} catch {
			setPlaying(false);
		}
	}

	function seek(nextTime: number) {
		const video = videoRef.current;
		if (!video || !Number.isFinite(nextTime)) return;
		video.currentTime = Math.max(0, Math.min(nextTime, duration || nextTime));
		setCurrentTime(video.currentTime);
	}

	function changeVolume(nextVolume: number) {
		const video = videoRef.current;
		if (!video) return;
		video.volume = nextVolume;
		video.muted = nextVolume === 0;
		setVolume(nextVolume);
		setMuted(video.muted);
		storePlayerPreferences(nextVolume, playbackRate);
	}

	function toggleMute() {
		const video = videoRef.current;
		if (!video) return;
		video.muted = !video.muted;
		setMuted(video.muted);
	}

	function changePlaybackRate(nextRate: number) {
		const video = videoRef.current;
		if (!video) return;
		video.playbackRate = nextRate;
		setPlaybackRate(nextRate);
		storePlayerPreferences(volume, nextRate);
	}

	function skip(seconds: number) {
		seek(currentTime + seconds);
		showControls();
	}

	async function toggleFullscreen() {
		if (!frameRef.current) return;
		try {
			if (document.fullscreenElement) await document.exitFullscreen();
			else await frameRef.current.requestFullscreen();
		} catch {
			// The browser may deny fullscreen outside a direct user gesture.
		}
	}

	async function togglePictureInPicture() {
		const video = videoRef.current as PictureInPictureVideo | null;
		const pictureDocument = document as PictureInPictureDocument;
		if (!video?.requestPictureInPicture) return;
		try {
			if (pictureDocument.pictureInPictureElement && pictureDocument.exitPictureInPicture) {
				await pictureDocument.exitPictureInPicture();
			} else {
				await video.requestPictureInPicture();
			}
		} catch {
			// Picture-in-picture availability can change with browser policy.
		}
	}

	function handleKeyboard(event: ReactKeyboardEvent<HTMLDivElement>) {
		const target = event.target as HTMLElement;
		if (["BUTTON", "INPUT", "SELECT"].includes(target.tagName)) return;
		switch (event.key.toLowerCase()) {
		case " ":
		case "k":
			event.preventDefault();
			void togglePlayback();
			break;
		case "arrowleft":
			event.preventDefault();
			skip(-10);
			break;
		case "arrowright":
			event.preventDefault();
			skip(10);
			break;
		case "m":
			toggleMute();
			break;
		case "f":
			void toggleFullscreen();
			break;
		}
	}

	function handleVideoClick() {
		if (videoClickTimerRef.current !== null) window.clearTimeout(videoClickTimerRef.current);
		videoClickTimerRef.current = window.setTimeout(() => {
			videoClickTimerRef.current = null;
			void togglePlayback();
		}, 180);
	}

	function handleVideoDoubleClick() {
		if (videoClickTimerRef.current !== null) {
			window.clearTimeout(videoClickTimerRef.current);
			videoClickTimerRef.current = null;
		}
		void toggleFullscreen();
	}

	function retryPlayback() {
		const video = videoRef.current;
		if (!video) return;
		setPlaybackError("");
		setLoading(true);
		setBuffering(false);
		video.load();
	}

	const progress = duration > 0 ? (currentTime / duration) * 100 : 0;
	const progressStyle = { "--video-progress": `${progress}%` } as CSSProperties;

	return (
		<div
			ref={frameRef}
			className={`eterea-video-player${playing ? " is-playing" : " is-paused"}${fullscreen ? " is-fullscreen" : ""}${controlsVisible ? "" : " controls-hidden"}`}
			tabIndex={0}
			onKeyDown={handleKeyboard}
			onMouseMove={() => showControls()}
			onPointerDown={() => showControls()}
			onFocusCapture={() => showControls()}
			onBlurCapture={(event) => {
				if (!event.currentTarget.contains(event.relatedTarget as Node | null)) scheduleControlHide();
			}}
			aria-label={`Video player for ${name}`}
		>
			<video
				ref={videoRef}
				src={url}
				preload="metadata"
				playsInline
				onClick={handleVideoClick}
				onDoubleClick={handleVideoDoubleClick}
				onLoadStart={() => {
					setLoading(true);
					setPlaybackError("");
				}}
				onLoadedMetadata={(event) => {
					const video = event.currentTarget;
					const preferences = readPlayerPreferences();
					video.volume = preferences.volume;
					video.playbackRate = preferences.playbackRate;
					setVolume(preferences.volume);
					setMuted(preferences.volume === 0);
					setPlaybackRate(preferences.playbackRate);
					setDuration(Number.isFinite(video.duration) ? video.duration : 0);
					setResolution(formatResolution(video.videoWidth, video.videoHeight));
					setPictureInPictureAvailable(Boolean((document as PictureInPictureDocument).pictureInPictureEnabled && (video as PictureInPictureVideo).requestPictureInPicture));
					setLoading(false);
				}}
				onDurationChange={(event) => setDuration(Number.isFinite(event.currentTarget.duration) ? event.currentTarget.duration : 0)}
				onTimeUpdate={(event) => setCurrentTime(event.currentTarget.currentTime)}
				onCanPlay={() => {
					setLoading(false);
					setBuffering(false);
				}}
				onWaiting={() => setBuffering(true)}
				onSeeking={() => setBuffering(true)}
				onSeeked={() => setBuffering(false)}
				onPlaying={() => {
					playingRef.current = true;
					setPlaying(true);
					setBuffering(false);
					showControls();
				}}
				onPause={() => {
					playingRef.current = false;
					setPlaying(false);
					setControlsVisible(true);
					clearControlTimer();
				}}
				onEnded={() => {
					playingRef.current = false;
					setPlaying(false);
					setControlsVisible(true);
				}}
				onError={(event) => {
					playingRef.current = false;
					setPlaying(false);
					setLoading(false);
					setBuffering(false);
					setControlsVisible(true);
					setPlaybackError(mediaErrorMessage(event.currentTarget.error?.code));
				}}
				onVolumeChange={(event) => {
					setVolume(event.currentTarget.volume);
					setMuted(event.currentTarget.muted);
				}}
			/>

			{(loading || buffering) && !playbackError && (
				<div className="video-status-overlay video-loading-state" role="status">
					<SpinnerGapIcon />
					<span>{loading ? "Loading video…" : "Buffering…"}</span>
				</div>
			)}
			{playbackError && (
				<div className="video-status-overlay video-error-state" role="alert">
					<WarningCircleIcon />
					<strong>Video preview unavailable</strong>
					<p>{playbackError} You can still download the original file below.</p>
					<button type="button" onClick={retryPlayback}>Try again</button>
				</div>
			)}

			<div className="video-source-badge"><span>Original</span>{resolution && <strong>{resolution}</strong>}</div>
			{!loading && !playbackError && (
				<button className="video-center-play" type="button" aria-label="Play video" onClick={() => void togglePlayback()}>
					<PlayIcon weight="fill" />
				</button>
			)}

			<div className="video-controls">
				<input
					className="video-scrubber"
					type="range"
					min="0"
					max={duration || 0}
					step="0.01"
					value={Math.min(currentTime, duration || 0)}
					style={progressStyle}
					onChange={(event) => seek(Number(event.target.value))}
					aria-label="Video position"
				/>
				<div className="video-control-row">
					<button type="button" aria-label={playing ? "Pause video" : "Play video"} onClick={() => void togglePlayback()}>
						{playing ? <PauseIcon weight="fill" /> : <PlayIcon weight="fill" />}
					</button>
					<button className="video-skip-button" type="button" aria-label="Skip back 10 seconds" onClick={() => skip(-10)}><ArrowCounterClockwiseIcon /><span>10</span></button>
					<button className="video-skip-button" type="button" aria-label="Skip forward 10 seconds" onClick={() => skip(10)}><ArrowClockwiseIcon /><span>10</span></button>
					<span className="video-time"><strong>{formatMediaTime(currentTime)}</strong><span>/</span>{formatMediaTime(duration)}</span>
					<div className="video-volume-control">
						<button type="button" aria-label={muted ? "Unmute video" : "Mute video"} onClick={toggleMute}>
							{muted || volume === 0 ? <SpeakerSlashIcon /> : <SpeakerHighIcon />}
						</button>
						<input type="range" min="0" max="1" step="0.05" value={muted ? 0 : volume} onChange={(event) => changeVolume(Number(event.target.value))} aria-label="Video volume" />
					</div>
					<span className="video-control-spacer" />
					<label className="video-speed-control">
						<span className="visually-hidden">Playback speed</span>
						<select value={playbackRate} onChange={(event) => changePlaybackRate(Number(event.target.value))} aria-label="Playback speed">
							<option value="0.5">0.5×</option>
							<option value="0.75">0.75×</option>
							<option value="1">1×</option>
							<option value="1.25">1.25×</option>
							<option value="1.5">1.5×</option>
							<option value="2">2×</option>
						</select>
					</label>
					{pictureInPictureAvailable && (
						<button type="button" aria-label="Open picture in picture" onClick={() => void togglePictureInPicture()}><PictureInPictureIcon /></button>
					)}
					<button type="button" aria-label={fullscreen ? "Exit fullscreen" : "Enter fullscreen"} onClick={() => void toggleFullscreen()}><CornersOutIcon /></button>
				</div>
			</div>
		</div>
	);
}

export function formatMediaTime(value: number) {
	if (!Number.isFinite(value) || value < 0) return "0:00";
	const totalSeconds = Math.floor(value);
	const hours = Math.floor(totalSeconds / 3600);
	const minutes = Math.floor((totalSeconds % 3600) / 60);
	const seconds = totalSeconds % 60;
	if (hours > 0) return `${hours}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
	return `${minutes}:${String(seconds).padStart(2, "0")}`;
}

function formatResolution(width: number, height: number) {
	if (!width || !height) return "";
	const longEdge = Math.max(width, height);
	const shortEdge = Math.min(width, height);
	if (shortEdge >= 2160 || longEdge >= 3840) return "4K";
	if (shortEdge >= 1440) return "1440p";
	if (shortEdge >= 1080) return "1080p";
	if (shortEdge >= 720) return "720p";
	if (shortEdge >= 480) return "480p";
	return `${width}×${height}`;
}

function readPlayerPreferences() {
	const fallback = { volume: 1, playbackRate: 1 };
	try {
		const stored = window.localStorage.getItem(PLAYER_PREFERENCES_KEY);
		if (!stored) return fallback;
		const parsed = JSON.parse(stored) as { volume?: unknown; playbackRate?: unknown };
		const storedVolume = typeof parsed.volume === "number" && parsed.volume >= 0 && parsed.volume <= 1 ? parsed.volume : fallback.volume;
		const storedRate = typeof parsed.playbackRate === "number" && PLAYBACK_RATES.includes(parsed.playbackRate) ? parsed.playbackRate : fallback.playbackRate;
		return { volume: storedVolume, playbackRate: storedRate };
	} catch {
		return fallback;
	}
}

function storePlayerPreferences(storedVolume: number, storedRate: number) {
	try {
		window.localStorage.setItem(PLAYER_PREFERENCES_KEY, JSON.stringify({ volume: storedVolume, playbackRate: storedRate }));
	} catch {
		// Private browsing or storage policy may make preferences unavailable.
	}
}

function mediaErrorMessage(code?: number) {
	switch (code) {
	case 2:
		return "The video was interrupted by a network problem.";
	case 3:
		return "The browser could not decode this video.";
	case 4:
		return "This browser does not support the video's codec or format.";
	default:
		return "The video could not be loaded.";
	}
}
