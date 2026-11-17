import type { Metadata } from "next";
import "@fontsource/archivo/700.css";
import "@fontsource/ibm-plex-sans/400.css";
import "@fontsource/ibm-plex-sans/500.css";
import "@fontsource/ibm-plex-mono/400.css";
import "@fontsource/ibm-plex-mono/500.css";
import "./globals.css";

// Feature: use the deployed domain for canonical/social URLs; never publish localhost as canonical.
const siteURL = process.env.SITE_URL;
const title = "QuickShortURL — URL Shortener & Custom Short Links";
const description =
  "Shorten long URLs into tiny, shareable links with QuickShortURL. Choose a custom ending, set link expiration, and copy your short URL in one click.";

export const metadata: Metadata = {
  title,
  description,
  applicationName: "QuickShortURL",
  keywords: [
    "URL shortener",
    "short URL",
    "shorten URL",
    "tiny links",
    "custom short links",
    "link expiration",
    "QuickShortURL",
  ],
  authors: [
    { name: "Prathamesh Thakare", url: "https://github.com/thakare18" },
  ],
  ...(siteURL
    ? { metadataBase: new URL(siteURL), alternates: { canonical: "/" } }
    : {}),
  openGraph: {
    title,
    description,
    siteName: "QuickShortURL",
    type: "website",
    locale: "en_US",
    ...(siteURL ? { url: new URL("/", siteURL).href } : {}),
  },
  twitter: { card: "summary", title, description },
  robots: { index: true, follow: true },
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
