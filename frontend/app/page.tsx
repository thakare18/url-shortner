"use client";

import { FormEvent, useEffect, useRef, useState } from "react";
import SiteFooter from "./components/site-footer";

type ShortLink = {
  id: string;
  original_url: string;
  short_url: string;
  creation_date: string;
  expires_at: string | null;
};
type Availability = {
  state: "idle" | "checking" | "available" | "taken" | "error";
  message: string;
};

function Arrow({ diagonal = false }: { diagonal?: boolean }) {
  return (
    <svg
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      aria-hidden="true"
    >
      <path d={diagonal ? "M6 18 18 6M6 6h12v12" : "M4 12h16m-6-6 6 6-6 6"} />
    </svg>
  );
}

export default function Home() {
  const [url, setURL] = useState("");
  const [alias, setAlias] = useState("");
  const [expiry, setExpiry] = useState("never");
  const [availability, setAvailability] = useState<Availability>({
    state: "idle",
    message: "Leave blank for an automatically generated link.",
  });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<ShortLink | null>(null);
  const [copied, setCopied] = useState(false);
  const [copyError, setCopyError] = useState("");
  const [publicBaseURL, setPublicBaseURL] = useState("");
  const destinationInput = useRef<HTMLInputElement>(null);
  const resultInput = useRef<HTMLInputElement>(null);
  const resultPanel = useRef<HTMLElement>(null);
  const copyButton = useRef<HTMLButtonElement>(null);
  const copyTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    // Feature: preview uses Go's configured public domain, exactly like the created link.
    const controller = new AbortController();
    fetch("/api/config", { signal: controller.signal })
      .then((response) => (response.ok ? response.json() : Promise.reject()))
      .then((config) => setPublicBaseURL(config.public_base_url))
      .catch(() => {}); // The shortening form still works if the optional preview cannot load.
    return () => {
      controller.abort();
      if (copyTimer.current) clearTimeout(copyTimer.current);
    };
  }, []);

  // Feature: after a successful submission, reveal the result and focus Copy without a second jump.
  useEffect(() => {
    if (!result) return;
    const frame = requestAnimationFrame(() => {
      copyButton.current?.focus({ preventScroll: true });
      resultPanel.current?.scrollIntoView({
        behavior: window.matchMedia("(prefers-reduced-motion: reduce)").matches
          ? "instant"
          : "smooth",
        block: "start",
      });
    });
    return () => cancelAnimationFrame(frame);
  }, [result]);

  // Feature: debounce availability checks and cancel stale requests while the user types.
  useEffect(() => {
    if (!alias) {
      setAvailability({
        state: "idle",
        message: "Leave blank for an automatically generated link.",
      });
      return;
    }
    if (!/^[a-zA-Z0-9][a-zA-Z0-9_-]{2,47}$/.test(alias)) {
      setAvailability({
        state: "error",
        message: "Use 3–48 letters, numbers, hyphens or underscores.",
      });
      return;
    }
    const controller = new AbortController();
    setAvailability({ state: "checking", message: "Checking availability…" });
    const timer = setTimeout(async () => {
      try {
        const response = await fetch(
          `/api/availability?slug=${encodeURIComponent(alias)}`,
          { signal: controller.signal },
        );
        const data = await response.json();
        if (!response.ok)
          throw new Error(data.error || "Could not check availability.");
        setAvailability({
          state: data.available ? "available" : "taken",
          message: data.available
            ? "Available — this ending is yours to use."
            : "Already taken. Try a different ending.",
        });
      } catch (error) {
        if (!controller.signal.aborted)
          setAvailability({
            state: "error",
            message:
              error instanceof Error
                ? error.message
                : "Could not check availability.",
          });
      }
    }, 350);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [alias]);

  // Feature: reset all draft/result feedback, then focus the URL field for the next link.
  function clearForm() {
    if (busy) return;
    if (copyTimer.current) clearTimeout(copyTimer.current);
    setURL("");
    setAlias("");
    setExpiry("never");
    setResult(null);
    setError("");
    setCopied(false);
    setCopyError("");
    destinationInput.current?.focus();
  }

  // Feature: native form submission supports both the button and Enter, with inline API errors.
  async function shorten(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    setBusy(true);
    setError("");
    setResult(null);
    setCopied(false);
    setCopyError("");
    try {
      const response = await fetch("/api/shorten", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          url: url.trim(),
          custom_alias: alias,
          expires_in: expiry,
        }),
      });
      const data = await response.json();
      if (!response.ok)
        throw new Error(data.error || "Could not shorten your URL.");
      setResult(data);
    } catch (error) {
      setError(
        error instanceof Error
          ? error.message
          : "Connection failed. Please try again.",
      );
    } finally {
      setBusy(false);
    }
  }

  // Feature: one-click copy confirms success only after the clipboard write succeeds.
  async function copy() {
    if (!result) return;
    setCopyError("");
    try {
      await navigator.clipboard.writeText(result.short_url);
      setCopied(true);
      if (copyTimer.current) clearTimeout(copyTimer.current);
      copyTimer.current = setTimeout(() => setCopied(false), 2500);
    } catch {
      resultInput.current?.select();
      setCopyError(
        "Copy unavailable. Your link is selected; use your device’s Copy command.",
      );
    }
  }

  return (
    <>
      <header className="nav">
        <div className="nav-inner">
          <a href="#" className="brand" aria-label="QuickShortURL home">
            <span className="monogram">
              q<span>↗</span>
            </span>
            <span>QuickShortURL</span>
          </a>
          <nav aria-label="Main navigation">
            <a href="#shortener">Shorten a link</a>
            <a href="#how-it-works">
              How it works <span>↗</span>
            </a>
          </nav>
          <div className="nav-right">
            <span className="version">
              V <b>1.0</b>
            </span>
            <a className="nav-cta" href="#destination">
              Get started <Arrow />
            </a>
          </div>
        </div>
      </header>

      <main className="container">
        <section className="hero" aria-labelledby="headline">
          <div>
            <p className="eyebrow">
              <i /> SPEC / 001 — LINKS, SIMPLIFIED
            </p>
            <h1 id="headline">
              Less link.
              <br />
              More possibility.
            </h1>
          </div>
          <div className="hero-aside">
            <p>
              Big ideas deserve better links.
              <br />
              Turn long URLs into short, memorable links.
              <br className="desktop-break" /> Make them yours. Share them
              anywhere.
            </p>
            <a href="#destination" className="text-link">
              One link. A shorter way there. <Arrow diagonal />
            </a>
            <p className="micro">NO ACCOUNT. NO FRICTION. JUST SHORTER.</p>
          </div>
        </section>

        <section
          id="shortener"
          className="workspace"
          aria-labelledby="form-title"
        >
          <div className="dimension">
            <span />
            <p>YOUR NEXT SHORTCUT</p>
            <span />
          </div>
          <span className="corner tl" />
          <span className="corner tr" />
          <span className="corner bl" />
          <span className="corner br" />
          <div className="panel-bar">
            <span className="panel-title">
              <span className="window-dots">
                <i />
                <i />
                <i />
              </span>{" "}
              quickshorturl / new-link
            </span>
            <span className="panel-meta">LONG URL → SHORT LINK</span>
          </div>
          <div className="work-grid">
            <aside className="rail">
              <div>
                <p className="micro">LINK WORKSPACE</p>
                <div className="rail-active">
                  <span>01</span> Create a link <Arrow />
                </div>
                <p className="rail-description">
                  A smaller link.
                  <br />
                  The same destination.
                </p>
              </div>
              <div className="rail-footer">
                <span className="link-symbol">↗</span>
                <p className="micro">BUILT FOR SHARING</p>
                <p>
                  Personal by design.
                  <br />
                  Simple by default.
                </p>
              </div>
            </aside>
            <div className="form-pane">
              <div className="form-heading">
                <h2 id="form-title">Let’s make it short.</h2>
                <span className="micro">[ NEW LINK ]</span>
              </div>
              <form onSubmit={shorten} aria-busy={busy}>
                {/* Feature: large, high-contrast labels and numbered badges make the three inputs easy to scan. */}
                <label
                  className="field-label input-label"
                  htmlFor="destination"
                >
                  <span className="field-step" aria-hidden="true">
                    01
                  </span>
                  <span className="field-name">
                    Destination URL — Paste your link here
                  </span>
                </label>
                <div className="url-row">
                  <input
                    ref={destinationInput}
                    id="destination"
                    name="url"
                    type="url"
                    required
                    maxLength={8192}
                    value={url}
                    onChange={(event) => setURL(event.target.value)}
                    placeholder="https://your-very-long-link.com/something-great"
                    autoComplete="url"
                    aria-describedby="url-hint"
                  />
                  <button className="primary" type="submit" disabled={busy}>
                    {busy ? "Shortening…" : "Shorten URL"}
                    <Arrow />
                  </button>
                </div>
                <p className="field-hint" id="url-hint">
                  Paste your long URL above. We’ll take care of the rest.
                </p>
                <div className="options-grid">
                  <div>
                    <label className="field-label input-label" htmlFor="alias">
                      <span className="field-step" aria-hidden="true">
                        02
                      </span>
                      <span className="field-name">Custom ending</span>
                      <span className="optional-tag">Optional</span>
                    </label>
                    <div className="alias-row">
                      {/* Feature: generic protocol label keeps the form independent of the hosting domain. */}
                      <span>HTTPS /</span>
                      <input
                        id="alias"
                        name="alias"
                        value={alias}
                        onChange={(event) => setAlias(event.target.value)}
                        placeholder="my-portfolio"
                        maxLength={48}
                        autoCapitalize="none"
                        autoCorrect="off"
                        spellCheck={false}
                        aria-describedby="alias-hint alias-preview"
                        aria-invalid={
                          result?.id !== alias &&
                          (availability.state === "taken" ||
                            (availability.state === "error" && !!alias))
                        }
                      />
                    </div>
                    <p
                      id="alias-hint"
                      className={`field-hint ${result?.id !== alias && (availability.state === "taken" || availability.state === "error") ? "invalid" : ""}`}
                      role="status"
                    >
                      {result?.id === alias
                        ? "Created — your custom link is ready below."
                        : availability.message}
                    </p>
                    {/* Feature: a non-clickable preview wraps long endings without suggesting an uncreated link is live. */}
                    {alias &&
                      result?.id !== alias &&
                      publicBaseURL &&
                      /^[a-zA-Z0-9][a-zA-Z0-9_-]{2,47}$/.test(alias) && (
                        <div id="alias-preview" className="alias-preview">
                          <span className="micro">LINK PREVIEW</span>
                          <span className="preview-url">
                            {publicBaseURL}/{alias}
                          </span>
                          <span className="field-hint">
                            Preview only · create the link to activate it.
                          </span>
                        </div>
                      )}
                  </div>
                  <div>
                    <label className="field-label input-label" htmlFor="expiry">
                      <span className="field-step" aria-hidden="true">
                        03
                      </span>
                      <span className="field-name">Link expiration</span>
                    </label>
                    <select
                      id="expiry"
                      name="expiry"
                      value={expiry}
                      onChange={(event) => setExpiry(event.target.value)}
                    >
                      <option value="never">Never expires</option>
                      <option value="1h">After 1 hour</option>
                      <option value="24h">After 24 hours</option>
                      <option value="7d">After 7 days</option>
                      <option value="30d">After 30 days</option>
                    </select>
                    <p className="field-hint">Your link, on your schedule.</p>
                  </div>
                </div>
                {error && (
                  <p className="error-box" role="alert">
                    {error}
                  </p>
                )}
                {!result && (url || alias || expiry !== "never") && (
                  <div className="form-actions">
                    <button
                      type="button"
                      className="secondary-button"
                      onClick={clearForm}
                      disabled={busy}
                    >
                      Clear form <span aria-hidden="true">↺</span>
                    </button>
                  </div>
                )}
              </form>
              {/* Feature: the result receipt shows real API data; long URLs wrap on small screens. */}
              {result ? (
                <section
                  ref={resultPanel}
                  className="receipt result"
                  aria-label="Your short link"
                  aria-live="polite"
                >
                  <div className="receipt-top">
                    <h3 className="result-title">Your short link is ready</h3>
                    <span className="stamp">CREATED</span>
                  </div>
                  <div className="result-row">
                    <input
                      ref={resultInput}
                      aria-label="Short URL"
                      value={result.short_url}
                      readOnly
                      onFocus={(event) => event.target.select()}
                    />
                    <button
                      ref={copyButton}
                      type="button"
                      className="copy-button"
                      onClick={copy}
                    >
                      <span aria-hidden="true">{copied ? "✓" : "⧉"}</span>
                      {copied ? "Copied" : "Copy"}
                    </button>
                    <a
                      className="open-link"
                      href={result.short_url}
                      target="_blank"
                      rel="noreferrer"
                      aria-label="Open short link"
                    >
                      <Arrow diagonal />
                    </a>
                  </div>
                  <p className="destination-preview">
                    <strong>Original URL:</strong> {result.original_url}
                  </p>
                  <p className="micro">
                    {result.expires_at
                      ? `EXPIRES ${new Date(result.expires_at).toLocaleString()}`
                      : "NO EXPIRATION"}{" "}
                    · READY TO SHARE
                  </p>
                  {copyError && (
                    <p role="alert" className="field-hint">
                      {copyError}
                    </p>
                  )}
                  <div className="result-actions">
                    <button
                      type="button"
                      className="secondary-button"
                      onClick={clearForm}
                      disabled={busy}
                    >
                      Create another link <Arrow />
                    </button>
                  </div>
                </section>
              ) : (
                <div className="receipt empty-receipt">
                  <div>
                    <span className="receipt-icon" aria-hidden="true">
                      ↳
                    </span>
                    <div>
                      <p className="field-label">A GOOD LINK STARTS HERE.</p>
                      <p>Your short link will appear here, ready to copy.</p>
                    </div>
                  </div>
                  <span className="micro">AWAITING INPUT</span>
                </div>
              )}
            </div>
          </div>
          <div className="panel-bottom">
            <span>HTTPS SUPPORTED</span>
            <span>
              SMALL LINK. SAME DESTINATION. <Arrow diagonal />
            </span>
          </div>
        </section>

        <section
          id="how-it-works"
          className="features"
          aria-label="How it works"
        >
          <article>
            <span className="feature-number">
              01<span> /</span>
            </span>
            <div className="accent-tick" />
            <h3>Paste. Shorten. Share.</h3>
            <p>
              From a long URL to a clean link in seconds. Copy it with a single
              click.
            </p>
          </article>
          <article>
            <span className="feature-number">
              02<span> /</span>
            </span>
            <div className="accent-tick" />
            <h3>Make it memorable.</h3>
            <p>
              Your portfolio, your project, your name. Choose an ending that
              feels like you.
            </p>
          </article>
          <article>
            <span className="feature-number">
              03<span> /</span>
            </span>
            <div className="accent-tick" />
            <h3>Set its lifetime.</h3>
            <p>
              A quick share or a lasting connection. Decide how long your link
              stays active.
            </p>
          </article>
        </section>
      </main>
      <div className="ticker">
        <span>LESS TO TYPE</span>
        <i />
        <span>MORE TO SHARE</span>
        <i />
        <span>YOUR LINKS, SIMPLIFIED</span>
        <i />
        <span>QUICKSHORTURL</span>
        <i />
        <span>BUILT FOR THE EVERYDAY</span>
      </div>
      <SiteFooter />
    </>
  );
}
