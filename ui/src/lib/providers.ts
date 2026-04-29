// Centralized provider configuration for icons, names, and help URLs

export const PROVIDER_NAMES: Record<string, string> = {
  github: "GitHub",
  gitlab: "GitLab",
  notion: "Notion",
  google_drive: "Google Drive",
  dropbox: "Dropbox",
  confluence: "Confluence",
  jira: "Jira",
  trello: "Trello",
  asana: "Asana",
  linear: "Linear",
  figma: "Figma",
  miro: "Miro",
  onedrive: "OneDrive",
  sharepoint: "SharePoint",
};

export const PROVIDER_ICONS: Record<string, string> = {
  github: "/logos/github/github_icon.png",
  gitlab: "/logos/gitlab/gitlab_logo.png",
  notion: "/logos/notion/notion_icon.png",
  google_drive: "/logos/google/google_drive_icon.png",
  dropbox: "/logos/dropbox/dropbox_icon.png",
  confluence: "/logos/atlassian/confluence.svg",
  jira: "/logos/atlassian/jira.svg",
  trello: "/logos/atlassian/trello-icon.png",
  asana: "/logos/asana/asana-icon.svg",
  linear: "/logos/linear/linear_icon.svg",
  figma: "/logos/figma/figma_icon.svg",
  miro: "/logos/miro/miro-icon.svg",
  slack: "/logos/slack/slack_icon.webp",
  onedrive: "/logos/microsoft/microsoft_onedrive_icon.png",
  sharepoint: "/logos/microsoft/microsoft_sharepoint.png",
};

export const PROVIDER_HELP_URLS: Record<string, string> = {
  github: "https://github.com/settings/developers",
  gitlab: "https://gitlab.com/-/user_settings/applications",
  notion: "https://www.notion.so/profile/integrations",
  confluence: "https://developer.atlassian.com/console/myapps/",
  jira: "https://developer.atlassian.com/console/myapps/",
  trello: "https://trello.com/power-ups/admin",
  google_drive: "https://console.cloud.google.com/apis/credentials",
  dropbox: "https://www.dropbox.com/developers/apps",
  onedrive: "https://portal.azure.com/#blade/Microsoft_AAD_RegisteredApps/ApplicationsListBlade",
  figma: "https://www.figma.com/developers/apps",
  asana: "https://app.asana.com/0/developer-console",
};

export function getProviderIcon(providerType: string): string {
  return PROVIDER_ICONS[providerType] || "/logos/github/github_icon.png";
}

export function getProviderHelpUrl(providerType: string): string | undefined {
  return PROVIDER_HELP_URLS[providerType];
}

export function getProviderName(providerType: string): string {
  return PROVIDER_NAMES[providerType] || providerType;
}

// Maps provider types to their OAuth platform
// Most providers use the same name for both, but Microsoft services share a platform
export const PROVIDER_TO_PLATFORM: Record<string, string> = {
  github: "github",
  notion: "notion",
  onedrive: "microsoft",
  sharepoint: "microsoft",
  // Add more mappings as needed
};

export function getProviderPlatform(providerType: string): string {
  return PROVIDER_TO_PLATFORM[providerType] || providerType;
}
