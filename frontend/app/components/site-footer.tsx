// Feature: one shared footer keeps policy access and developer attribution consistent across pages.
export default function SiteFooter() {
  return (
    <footer className="container footer">
      <span>© {new Date().getFullYear()} QuickShortURL</span>
      <a className="footer-policy-link" href="/legal">
        Terms of Service &amp; Privacy Policy
      </a>
      <span className="developer-credit">
        Designed &amp; developed by{" "}
        <a
          href="https://github.com/thakare18"
          target="_blank"
          rel="noopener noreferrer"
        >
          Prathamesh Thakare
        </a>
      </span>
    </footer>
  );
}
