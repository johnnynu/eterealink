"use client";

import { useEffect, useState } from "react";
import { FileIcon } from "@/components/icons";
import { VideoPlayer } from "@/components/video-player";
import type { FilePreviewRecord } from "@/lib/types";

type FilePreviewProps = {
	name: string;
	preview?: FilePreviewRecord;
};

export function FilePreview({ name, preview }: FilePreviewProps) {
	if (!preview) {
		return (
			<div className="file-preview file-preview-fallback" role="status">
				<span><FileIcon /></span>
				<strong>Preview unavailable</strong>
				<p>This file type can still be downloaded safely.</p>
			</div>
		);
	}

	if (preview.kind === "image") {
		return (
			<div className="file-preview file-preview-image">
				{/* Signed object URLs are short-lived and cannot use Next's image optimizer. */}
				{/* eslint-disable-next-line @next/next/no-img-element */}
				<img src={preview.url} alt={`Preview of ${name}`} />
			</div>
		);
	}
	if (preview.kind === "pdf") {
		return (
			<div className="file-preview file-preview-document">
				<iframe src={preview.url} title={`Preview of ${name}`} referrerPolicy="no-referrer" />
			</div>
		);
	}
	if (preview.kind === "video") {
		return <div className="file-preview file-preview-video"><VideoPlayer name={name} url={preview.url} /></div>;
	}
	if (preview.kind === "audio") {
		return (
			<div className="file-preview file-preview-audio">
				<audio src={preview.url} controls preload="metadata" aria-label={`Preview of ${name}`} />
			</div>
		);
	}
	return <TextPreview name={name} url={preview.url} />;
}

function TextPreview({ name, url }: { name: string; url: string }) {
	const [state, setState] = useState<"loading" | "ready" | "failed">("loading");
	const [contents, setContents] = useState("");

	useEffect(() => {
		const controller = new AbortController();
		fetch(url, { cache: "no-store", signal: controller.signal })
			.then((response) => {
				if (!response.ok) throw new Error("preview request failed");
				return response.text();
			})
			.then((text) => {
				setContents(text);
				setState("ready");
			})
			.catch((error: unknown) => {
				if (error instanceof DOMException && error.name === "AbortError") return;
				setState("failed");
			});
		return () => controller.abort();
	}, [url]);

	if (state === "loading") return <div className="file-preview file-preview-text preview-loading" role="status">Loading text preview…</div>;
	if (state === "failed") {
		return (
			<div className="file-preview file-preview-fallback" role="status">
				<span><FileIcon /></span>
				<strong>Preview could not be loaded</strong>
				<p>The file is still available to download.</p>
			</div>
		);
	}
	return <pre className="file-preview file-preview-text" aria-label={`Preview of ${name}`}>{contents}</pre>;
}
