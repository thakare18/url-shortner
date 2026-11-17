import type { MetadataRoute } from "next";

// Feature: list public information pages, never users' short links or an invented domain.
export default function sitemap(): MetadataRoute.Sitemap {
  const siteURL = process.env.SITE_URL;
  return siteURL
    ? ["/", "/legal"].map((path) => ({ url: new URL(path, siteURL).href }))
    : [];
}
