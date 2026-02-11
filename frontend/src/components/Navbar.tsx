'use client';

import Link from 'next/link';
import { ConnectButton } from '@rainbow-me/rainbowkit';
import { Rocket, BarChart3, ShieldCheck, Gamepad2 } from 'lucide-react';
import { usePathname } from 'next/navigation';
import { cn } from '@/lib/utils';

export function Navbar() {
  const pathname = usePathname();

  const navItems = [
    { name: 'Home', href: '/', icon: Rocket },
    { name: 'Strategies', href: '/dashboard/strategies', icon: Gamepad2 },
    { name: 'Dashboard', href: '/dashboard', icon: BarChart3 },
  ];

  return (
    <header className="sticky top-0 z-50 w-full border-b border-white/10 bg-[#101116]/80 backdrop-blur-xl">
      <div className="container mx-auto px-4 h-16 flex items-center justify-between">
        {/* Logo */}
        <Link href="/" className="flex items-center gap-2 group">
           <div className="relative w-8 h-8 flex items-center justify-center bg-transparent">
              <Rocket className="w-8 h-8 text-primary group-hover:animate-pulse-fast transition-all" />
           </div>
           <span className="text-2xl font-bold tracking-tighter bg-clip-text text-transparent bg-gradient-to-r from-purple-400 to-pink-500 group-hover:from-purple-300 group-hover:to-pink-400">
             PumpPilot
           </span>
        </Link>

        {/* Desktop Nav */}
        <nav className="hidden md:flex items-center gap-6">
          {navItems.map((item) => {
            const isActive = pathname === item.href;
            const Icon = item.icon;
            return (
              <Link
                key={item.href}
                href={item.href}
                className={cn(
                  "flex items-center gap-2 text-sm font-medium transition-colors hover:text-white",
                  isActive ? "text-primary" : "text-muted-foreground"
                )}
              >
                <Icon className="w-4 h-4" />
                {item.name}
              </Link>
            );
          })}
        </nav>

        {/* Actions */}
        <div className="flex items-center gap-4">
           {/* Mock "Create Coin" Button */}
           <Link href="/create" className="hidden sm:flex items-center gap-2 px-4 py-2 rounded-full bg-secondary/20 hover:bg-secondary/30 text-secondary border border-secondary/50 transition-all font-bold text-sm">
             [start a new coin]
           </Link>

           <ConnectButton 
             accountStatus="address"
             chainStatus="icon"
             showBalance={false}
           />
        </div>
      </div>
    </header>
  );
}
