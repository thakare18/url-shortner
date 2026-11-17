import type { MetadataRoute } from "next";

// Feature: let search engines discover the homepage, without indexing API and user redirect paths.
export default function robots(): MetadataRoute.Robots {
  const siteURL = process.env.SITE_URL;
  return {
    rules: { userAgent: "*", allow: "/", disallow: ["/api/", "/s/"] },
    ...(siteURL ? { sitemap: new URL("/sitemap.xml", siteURL).href } : {}),
  };
}
