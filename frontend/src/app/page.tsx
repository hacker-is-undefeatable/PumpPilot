import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { ArrowRight, Flame, TrendingUp, Zap, Shield, Users } from "lucide-react";

export default function Home() {
  const trendingCoins = [
    { name: "PUMP", symbol: "PUMP", price: "$0.0042", change: "+420%", image: "🚀" },
    { name: "PEPE2", symbol: "PEPE", price: "$0.0001", change: "+69%", image: "🐸" },
    { name: "DOGE3", symbol: "DOGE", price: "$0.12", change: "+12%", image: "🐕" },
    { name: "MOON", symbol: "MOON", price: "$1.20", change: "+1000%", image: "🌕" },
  ];

  return (
    <div className="flex flex-col items-center gap-12 py-8">
      {/* Hero Section */}
      <section className="w-full max-w-5xl flex flex-col items-center text-center gap-6 mt-10">
        <h1 className="text-5xl md:text-7xl font-extrabold tracking-tight">
          <span className="text-white">Fair Launch</span> <br />
          <span className="bg-clip-text text-transparent bg-gradient-to-r from-purple-400 via-pink-500 to-orange-500">
             Meme Coins Instantly
          </span>
        </h1>
        <p className="text-xl text-muted-foreground max-w-2xl">
          The safest and most fun way to launch and trade tokens on Base. No presale, no team allocation, fair bonding curve.
        </p>
        <div className="flex gap-4 mt-4">
           <Button size="lg" variant="pump" className="text-lg px-8 h-14">
             Start a new coin
           </Button>
           <Button size="lg" variant="neon" className="text-lg px-8 h-14" asChild>
             <Link href="/dashboard/strategies">Trade Now</Link>
           </Button>
        </div>
      </section>

      {/* Stats Bar */}
      <div className="w-full max-w-4xl grid grid-cols-1 md:grid-cols-3 gap-6 bg-white/5 border border-white/10 rounded-2xl p-6 backdrop-blur-sm">
         <div className="text-center">
            <h3 className="text-3xl font-bold text-green-400">$4.2M</h3>
            <p className="text-sm text-gray-400">Total Volume (24h)</p>
         </div>
         <div className="text-center border-l border-white/10">
            <h3 className="text-3xl font-bold text-purple-400">1,203</h3>
            <p className="text-sm text-gray-400">Coins Launched</p>
         </div>
         <div className="text-center border-l border-white/10">
            <h3 className="text-3xl font-bold text-pink-400">Fair</h3>
            <p className="text-sm text-gray-400">Bonding Curve</p>
         </div>
      </div>

      {/* Trending Grid */}
      <section className="w-full max-w-6xl">
        <div className="flex items-center justify-between mb-6">
           <h2 className="text-3xl font-bold flex items-center gap-2">
             <Flame className="text-orange-500 fill-orange-500" /> 
             Trending Now
           </h2>
           <Link href="/dashboard" className="text-primary hover:underline">View All</Link>
        </div>
        
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
           {trendingCoins.map((coin, i) => (
             <Card key={i} className="bg-[#1e2029] border-white/10 hover:border-primary/50 transition-all hover:-translate-y-1 cursor-pointer group">
               <CardHeader className="flex flex-row items-center justify-between pb-2">
                 <div className="text-3xl bg-secondary/10 p-2 rounded-lg">{coin.image}</div>
                 <div className="text-right">
                    <div className="font-bold text-green-400">{coin.change}</div>
                 </div>
               </CardHeader>
               <CardContent>
                 <div className="font-bold text-lg text-white group-hover:text-primary transition-colors">
                   {coin.name} <span className="text-gray-500 text-sm">/ {coin.symbol}</span>
                 </div>
                 <div className="text-sm text-gray-400 mt-1">Market Cap: {coin.price}</div>
               </CardContent>
               <CardFooter className="pt-0">
                  <div className="text-xs text-gray-500">Created by 0x12...34</div>
               </CardFooter>
             </Card>
           ))}
        </div>
      </section>

      {/* Strategies Teaser */}
      <section className="w-full max-w-5xl py-12">
         <div className="text-center mb-10">
            <h2 className="text-3xl font-bold mb-4">Trade Smarter with Pump Strategies</h2>
            <p className="text-gray-400">Choose your playstyle or automate your trades.</p>
         </div>
         <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <Card className="bg-black/40 border-purple-500/30">
               <CardHeader><Zap className="w-10 h-10 text-yellow-400 mb-2" /><CardTitle>Sniper Mode</CardTitle></CardHeader>
               <CardContent className="text-sm text-gray-400">Be the first to buy new bonding curves as soon as they launch.</CardContent>
            </Card>
            <Card className="bg-black/40 border-blue-500/30">
               <CardHeader><Users className="w-10 h-10 text-blue-400 mb-2" /><CardTitle>Copy Trade</CardTitle></CardHeader>
               <CardContent className="text-sm text-gray-400">Automatically follow profitable wallets and replicate their buys.</CardContent>
            </Card>
            <Card className="bg-black/40 border-green-500/30">
               <CardHeader><Shield className="w-10 h-10 text-green-400 mb-2" /><CardTitle>Risk Guard</CardTitle></CardHeader>
               <CardContent className="text-sm text-gray-400">Auto-sell mechanisms to protect your gains and limit losses.</CardContent>
            </Card>
         </div>
      </section>
    </div>
  );
}
