// Check result tiers (must match backend selectCheckResultTier)
export function checkTierLabel(fromTs, toTs) {
  const hours = (toTs - fromTs) / 3600;
  if (hours <= 6) return 'raw';
  if (hours <= 168) return 'hourly';  // 7 days
  return 'daily';
}

// Agent metric tiers (must match backend selectAgentMetricTier)
export function metricTierLabel(fromTs, toTs) {
  const hours = (toTs - fromTs) / 3600;
  if (hours <= 1) return 'raw';
  if (hours <= 6) return '5min';
  if (hours <= 48) return '15min';
  if (hours <= 168) return 'hourly';  // 7 days
  return 'daily';
}

// Preset time ranges (value = seconds back from now)
export const timeRanges = [
  { label: '1h',  value: 3600 },
  { label: '6h',  value: 21600 },
  { label: '24h', value: 86400 },
  { label: '48h', value: 172800 },
  { label: '7d',  value: 604800 },
  { label: '30d', value: 2592000 },
  { label: '90d', value: 7776000 },
  { label: '1y',  value: 31536000 },
];
