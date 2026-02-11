import { getDefaultConfig } from '@rainbow-me/rainbowkit';
import { base, mainnet, optimism } from 'wagmi/chains';

export const config = getDefaultConfig({
  appName: 'PumpPilot',
  projectId: 'YOUR_WALLETCONNECT_PROJECT_ID',
  chains: [base, mainnet, optimism],
  ssr: true, // If your dApp uses server side rendering (SSR)
});
