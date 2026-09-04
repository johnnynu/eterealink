import { UploadWorkspace } from "@/components/upload-workspace";

export default function Home() {
  return (
    <div className="home-page">
      <div className="ambient ambient-one" />
      <div className="ambient ambient-two" />
      <section className="hero">
        <div className="hero-copy">
          <p className="hero-kicker"><span /> Files travel light</p>
          <h1>Share your files.<br /><em>Leave no clutter.</em></h1>
          <p className="hero-description">A direct, private handoff with no account and no inbox. Your link quietly disappears after 24 hours.</p>
          <div className="promise-row" aria-label="Transfer features">
            <div><strong>Direct</strong><span>Browser to storage</span></div>
            <div><strong>Private</strong><span>Short-lived access</span></div>
            <div><strong>Temporary</strong><span>24-hour links</span></div>
          </div>
        </div>
        <UploadWorkspace />
      </section>
      <section className="how-it-works" aria-labelledby="how-title">
        <div>
          <p className="eyebrow">Simple by design</p>
          <h2 id="how-title">One link. Three steps.</h2>
        </div>
        <ol>
          <li><span>01</span><strong>Choose</strong><p>Pick up to 10 files, with a combined limit of 1 GB. No sign-up required.</p></li>
          <li><span>02</span><strong>Send</strong><p>Your browser transfers them directly to private object storage.</p></li>
          <li><span>03</span><strong>Share</strong><p>Copy one link. Recipients can download a ZIP or individual files for 24 hours.</p></li>
        </ol>
      </section>
    </div>
  );
}
