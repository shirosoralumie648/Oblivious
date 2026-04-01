export type ApiUser = {
  id: string;
  email: string;
};

export type ConversationSummary = {
  id: string;
  title: string;
};

export type UsageSummary = {
  period: string;
  requests: number;
};
