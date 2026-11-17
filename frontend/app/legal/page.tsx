import type { Metadata } from "next";
import SiteFooter from "../components/site-footer";

const siteURL = process.env.SITE_URL;
const title = "Privacy Policy & Terms of Service — QuickShortURL";
const description =
  "How QuickShortURL protects and stores short-link data, your privacy choices, service terms, and developer contact details.";
export const metadata: Metadata = {
  title,
  description,
  ...(siteURL ? { alternates: { canonical: "/legal" } } : {}),
  openGraph: {
    title,
    description,
    type: "website",
    siteName: "QuickShortURL",
    ...(siteURL ? { url: new URL("/legal", siteURL).href } : {}),
  },
  twitter: { card: "summary", title, description },
};

// Feature: both policies are readable without JavaScript; direct section links work on mobile and desktop.
export default function LegalPage() {
  return (
    <>
      <header className="nav">
        <div className="nav-inner legal-nav">
          <a href="/" className="brand" aria-label="QuickShortURL home">
            <span className="monogram">
              q<span>↗</span>
            </span>
            <span>QuickShortURL</span>
          </a>
          <a className="nav-cta" href="/#destination">
            Back to shortener <span aria-hidden="true">↗</span>
          </a>
        </div>
      </header>
      <main className="container legal-main">
        <div className="legal-intro">
          <p className="eyebrow">
            <i /> TRUST / PRIVACY &amp; TERMS
          </p>
          <h1>
            Clear links.
            <br />
            Clear policies.
          </h1>
          <p>
            Understand what we store, how we protect it, and what to expect from
            QuickShortURL.
          </p>
          <p className="micro">
            LAST UPDATED / <time dateTime="2026-11-17">17 NOVEMBER 2026</time>
          </p>
        </div>
        <nav className="policy-options" aria-label="Choose a policy">
          <a href="#privacy">
            <span className="micro">01 / YOUR DATA</span>
            <strong>Privacy Policy</strong>
            <span aria-hidden="true">↘</span>
          </a>
          <a href="#terms">
            <span className="micro">02 / USING THE SERVICE</span>
            <strong>Terms of Service</strong>
            <span aria-hidden="true">↘</span>
          </a>
        </nav>
        <div className="privacy-summary" aria-label="Privacy at a glance">
          <span>ENCRYPTED LINKS</span>
          <span>NO AD TRACKING</span>
          <span>CHOOSE YOUR EXPIRY</span>
        </div>

        {/* Feature: plain-language policies describe real data practices without exposing implementation details. */}
        <article id="privacy" className="policy-document" aria-labelledby="privacy-title">
          <p className="micro">POLICY / 01</p>
          <h2 id="privacy-title">Privacy Policy</h2>
          <p>QuickShortURL, developed by Prathamesh Thakare, uses only the information needed to create and operate your short links. We do not sell, rent, or trade your link data.</p>
          <h3>Your links and privacy</h3>
          <p>Your original link is stored in encrypted form, along with basic details such as its short link, creation date, and chosen expiry. This lets shared links keep working. No account is required.</p>
          <p>We do not use advertising trackers or build visitor profiles. Service providers help operate the website and may keep basic connection and security logs. Websites you visit through a short link have their own privacy policies.</p>
          <h3>How long information stays</h3>
          <p>Expired links stop working, and their stored destinations are automatically removed shortly afterwards; removal can take time. Links set to “Never expires” remain until removed. Copying a link or closing the tab does not delete it.</p>
          <p>A minimal record may remain indefinitely to prevent an old short link from being assigned to someone else. Copies kept for backup or security purposes may have different retention periods.</p>
          <h3>Your choices</h3>
          <p>Anyone with your active short link can open its destination. Avoid shortening links containing confidential information. We take precautions to protect stored links, but no online service can promise complete security.</p>
          <p>For privacy questions or a deletion request, email <a href="mailto:prathameshthakare9677@gmail.com">prathameshthakare9677@gmail.com</a>. We may need to verify your request. Your contact details and message are used to respond; please do not send passwords or other secrets.</p>
        </article>

        <article id="terms" className="policy-document" aria-labelledby="terms-title">
          <p className="micro">POLICY / 02</p>
          <h2 id="terms-title">Terms of Service</h2>
          <p>By using QuickShortURL, you agree to use it responsibly and follow applicable laws. You are responsible for the links you create and share.</p>
          <h3>Acceptable use</h3>
          <p>Do not use this service for scams, harmful software, spam, unlawful content, or attempts to disrupt the website. We may remove abusive links or restrict access when necessary.</p>
          <h3>Using short links</h3>
          <p>Custom endings depend on availability. Expired or removed endings may remain unavailable. We do not control destination websites, and a short link does not mean we endorse or guarantee that website.</p>
          <h3>Service availability</h3>
          <p>This is a showcase/demo project, provided “as is” and “as available.” Features and availability may change, and links can expire or become unavailable. Keep your original links and do not rely on this demo for critical access.</p>
          <p>To the extent permitted by law, we do not guarantee uninterrupted service and are not liable for losses caused by interruptions, third-party websites, or misuse. Your rights under applicable law remain unaffected.</p>
          <h3>Updates and contact</h3>
          <p>Changes to these terms or the privacy policy will be posted here with an updated date. For questions, contact <a href="mailto:prathameshthakare9677@gmail.com">prathameshthakare9677@gmail.com</a>.</p>
        </article>

        <aside className="legal-contact" aria-labelledby="contact-title">
          <p className="micro">A DIRECT LINE / DEVELOPER CONTACT</p>
          <h2 id="contact-title">Questions, feedback, or an opportunity?</h2>
          <p>
            Reach Prathamesh Thakare for privacy questions, support,
            collaboration, freelance work, or hiring opportunities.
          </p>
          <a className="secondary-button" href="mailto:prathameshthakare9677@gmail.com">
            prathameshthakare9677@gmail.com <span aria-hidden="true">↗</span>
          </a>
        </aside>
      </main>
      <SiteFooter />
    </>
  );
}
