'use client';

import { useState } from 'react';
import { supabase } from '@/lib/supabaseClient';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { useAccount, useSignMessage } from 'wagmi';
import { Wallet } from 'lucide-react';

export default function AuthPage() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const { address, isConnected } = useAccount();
  const { signMessageAsync } = useSignMessage();

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    const { error } = await supabase.auth.signInWithPassword({
      email,
      password,
    });
    if (error) alert(error.message);
    else window.location.href = '/dashboard';
    setLoading(false);
  };

  const handleSignUp = async () => {
    setLoading(true);
    const { error } = await supabase.auth.signUp({
      email,
      password,
    });
    if (error) alert(error.message);
    else alert('Check your email for the confirmation link!');
    setLoading(false);
  };

  const handleWeb3Login = async () => {
    if (!isConnected || !address) {
        alert("Please connect your wallet first via the button in the top right!");
        return;
    }

    try {
        const message = `Sign this message to authenticate with PumpPilot.\nNonce: ${Date.now()}`;
        const signature = await signMessageAsync({ message });
        
        // Here you would verify the signature on the backend/Supabase Edge Function 
        // to issue a session token.
        // For this demo, we'll pretend it worked.
        console.log("Signed:", signature);
        alert("Web3 Login Signature verified! (Simulated)");
        // In a real app: window.location.href = '/dashboard';
        
    } catch (err: any) {
        alert("Login failed: " + err.message);
    }
  };

  return (
    <div className="flex justify-center items-center min-h-[calc(100vh-100px)]">
      <Card className="w-full max-w-md bg-[#1e2029] border-white/10 shadow-2xl">
        <CardHeader className="text-center">
          <CardTitle className="text-3xl font-extrabold pump-text">Welcome Back</CardTitle>
          <CardDescription>Login to manage your strategies</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleLogin} className="space-y-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">Email</label>
              <Input 
                type="email" 
                placeholder="degen@pumppilot.com" 
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="bg-black/50 border-white/20"
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Password</label>
              <Input 
                type="password" 
                placeholder="••••••••" 
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="bg-black/50 border-white/20"
              />
            </div>
            
            <div className="flex gap-2">
                <Button type="submit" className="w-full bg-primary hover:bg-primary/80 text-black font-bold" disabled={loading}>
                {loading ? 'Loading...' : 'Log In'}
                </Button>
                <Button type="button" variant="outline" className="w-full border-white/20 hover:bg-white/10" onClick={handleSignUp} disabled={loading}>
                Sign Up
                </Button>
            </div>
          </form>

          <div className="relative my-6">
            <div className="absolute inset-0 flex items-center">
              <span className="w-full border-t border-white/10" />
            </div>
            <div className="relative flex justify-center text-xs uppercase">
              <span className="bg-[#1e2029] px-2 text-muted-foreground">Or continue with</span>
            </div>
          </div>

          <Button 
            variant="outline" 
            className="w-full h-12 border-purple-500/50 hover:bg-purple-500/10 hover:text-purple-300 gap-2 group"
            onClick={handleWeb3Login}
          >
            <Wallet className="w-5 h-5 group-hover:text-purple-400" />
            Sign in with Wallet
          </Button>

        </CardContent>
      </Card>
    </div>
  );
}
