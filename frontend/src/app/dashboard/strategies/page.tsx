'use client';

import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "lucide-react"; // Wait, Badge is ui
import { Zap, Users, Shield, TrendingUp, DollarSign, Crosshair, Sparkles } from "lucide-react";
import { cn } from "@/lib/utils";

// Mock Data
const strategies = [
  {
    id: 'manual',
    title: 'Manual Trading',
    description: 'Classic Pump.fun experience. Buy and sell manually on the bonding curve.',
    icon: TrendingUp,
    risk: 'Medium',
    color: 'text-blue-400',
    border: 'border-blue-500/20'
  },
  {
    id: 'sniper',
    title: 'Bonding Curve Sniper',
    description: 'Auto-buy new token launches in the first block. Highest risk, highest reward.',
    icon: Crosshair,
    risk: 'High',
    color: 'text-red-400',
    border: 'border-red-500/20'
  },
  {
    id: 'copy',
    title: 'Copy Trading',
    description: 'Replicate trades from top performing wallets automatically.',
    icon: Users,
    risk: 'Variable',
    color: 'text-purple-400',
    border: 'border-purple-500/20'
  },
  {
    id: 'dca',
    title: 'DCA Accumulator',
    description: 'Buy small amounts over time to smooth out volatility.',
    icon: DollarSign,
    risk: 'Low',
    color: 'text-green-400',
    border: 'border-green-500/20'
  },
  {
    id: 'momentum',
    title: 'Momentum Bot',
    description: 'Buys when volume spikes and sells when it drops.',
    icon: Zap,
    risk: 'High',
    color: 'text-yellow-400',
    border: 'border-yellow-500/20'
  },
  {
    id: 'hunter',
    title: 'Meme Hunter',
    description: 'AI-driven sentiment analysis to find the next GEM.',
    icon: Sparkles,
    risk: 'High',
    color: 'text-pink-400',
    border: 'border-pink-500/20'
  }
];

export default function StrategiesPage() {
  return (
    <div className="flex flex-col gap-8 py-8">
      <div className="flex flex-col gap-2">
         <h1 className="text-4xl font-bold tracking-tight">Select Strategy</h1>
         <p className="text-muted-foreground">Choose how you want to interact with the market. You can run multiple strategies at once.</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {strategies.map((strategy) => {
            const Icon = strategy.icon;
            return (
                <Card key={strategy.id} className={cn("bg-[#1e2029] hover:bg-[#252836] transition-all duration-300 hover:scale-[1.02] cursor-pointer border", strategy.border)}>
                    <CardHeader>
                        <div className="flex justify-between items-start">
                           <div className={cn("p-2 rounded-lg bg-black/20", strategy.color)}>
                              <Icon className="w-8 h-8" />
                           </div>
                           <span className={cn("text-xs font-bold px-2 py-1 rounded bg-black/40 border border-white/5", strategy.risk === 'High' ? 'text-red-400' : 'text-green-400')}>
                                {strategy.risk} Risk
                           </span>
                        </div>
                        <CardTitle className="mt-4 text-xl">{strategy.title}</CardTitle>
                    </CardHeader>
                    <CardContent>
                        <p className="text-sm text-gray-400 leading-relaxed">
                            {strategy.description}
                        </p>
                    </CardContent>
                    <CardFooter>
                        <Button className="w-full bg-white/5 hover:bg-white/10 text-white font-semibold border border-white/10">
                            Select & Configure
                        </Button>
                    </CardFooter>
                </Card>
            )
        })}
      </div>
    </div>
  );
}
