# 🚀 PumpPilot

PumpPilot is a modern Web3 dashboard application built for managing and monitoring DeFi strategies. It leverages the power of Next.js for the frontend, Supabase for backend services, and Wagmi/RainbowKit for seamless blockchain interactions.

## 🛠 Tech Stack

- **Framework:** [Next.js 16 (App Router)](https://nextjs.org/)
- **Language:** TypeScript
- **Styling:** Tailwind CSS 4, Framer Motion
- **Web3 Integration:** 
  - [Wagmi](https://wagmi.sh/) (React Hooks for Ethereum)
  - [RainbowKit](https://www.rainbowkit.com/) (Wallet connection)
  - [Viem](https://viem.sh/) (Ethereum Interface)
- **Backend & Auth:** [Supabase](https://supabase.com/)
- **State Management:** React Query (TanStack Query)
- **Icons:** Lucide React

## 📂 Project Structure

```
PumpPilot/
├── frontend/             # Main Next.js application
│   ├── src/
│   │   ├── app/          # App Router pages (Dashboard, Auth, etc.)
│   │   ├── components/   # Reusable UI components
│   │   ├── lib/          # Utilities, Supabase & Wagmi config
│   └── ...
```

## 🏎️ Getting Started

### Prerequisites

- Node.js (v18+ recommended)
- A [Supabase](https://supabase.com/) project
- A [WalletConnect](https://cloud.walletconnect.com/) Project ID

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/pumppilot.git
   cd pumppilot
   ```

2. Navigate to the frontend directory:
   ```bash
   cd frontend
   ```

3. Install dependencies:
   ```bash
   npm install
   # or
   yarn install
   # or
   pnpm install
   ```

### 🔐 Configuration

Create a `.env.local` file in the `frontend` directory and add your environment variables:

```env
# Supabase Configuration
NEXT_PUBLIC_SUPABASE_URL=your_supabase_project_url
NEXT_PUBLIC_SUPABASE_ANON_KEY=your_supabase_anon_key

# WalletConnect (for RainbowKit)
NEXT_PUBLIC_WALLETCONNECT_PROJECT_ID=your_walletconnect_project_id
```

*Note: You may need to update `src/lib/wagmi.ts` to use `process.env.NEXT_PUBLIC_WALLETCONNECT_PROJECT_ID` if it is currently hardcoded.*

### 🚀 Running the App

Start the development server:

```bash
npm run dev
```

Open [http://localhost:3000](http://localhost:3000) to view the application.

## ✨ Features

- **Authentication**: Secure user login via Supabase.
- **Web3 Wallet Connection**: Connect seamlessly with MetaMask, Rainbow, Coinbase Wallet, etc.
- **Dashboard**: Centralized view for monitoring activities.
- **Strategy Management**: Create and track various crypto strategies (`/dashboard/strategies`).
- **Responsive Design**: Mobile-friendly interface built with Tailwind CSS.

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## 📄 License

This project is open source and available under the [MIT License](LICENSE).

## Backend
See `backend/README.md` for backend setup and usage.
