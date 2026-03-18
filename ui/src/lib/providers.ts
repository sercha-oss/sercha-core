// Centralized provider configuration for icons and help URLs

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
  figma: "/logos/figma/figma_icon.svg",
  onedrive: "/logos/microsoft/microsoft_onedrive_icon.png",
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
