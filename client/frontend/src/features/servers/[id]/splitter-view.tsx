'use client';
import { useState } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Separator } from '@/components/ui/separator';
import { Button } from '@/components/ui/button';
import { useServers } from '@/hooks/use-servers';
import { Cpu, MemoryStick, HardDrive, ServerIcon, Plus } from 'lucide-react';
import { Dialog, DialogContent, DialogTrigger } from '@/components/ui/dialog';
import { CreateServerStepper } from '../create-server-stepper';

type SplitterViewProps = {
  serverId: string;
};

export default function SplitterView({ serverId }: SplitterViewProps) {
  const { servers: allServers, refresh } = useServers();
  const [isDialogOpen, setIsDialogOpen] = useState(false);

  const currentServerInfo = allServers.find(s => s.id === serverId);
  const subServers = allServers.filter(s => s.parentServerId === serverId);

  const usedCpuBySubservers = subServers.reduce((acc, s) => acc + (s.totalCpu || 0), 0);
  const usedMemoryBySubservers = subServers.reduce((acc, s) => acc + (s.totalMemory || 0), 0);
  const usedDiskBySubservers = subServers.reduce((acc, s) => acc + (s.totalDisk || 0), 0);

  // Un servidor hijo no puede tener subservidores a su vez (1 nivel de anidación)
  if (currentServerInfo?.parentServerId) {
    return (
      <div className="mt-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
        <Card className="border-0 bg-transparent shadow-none">
          <CardHeader className="px-0">
            <CardTitle className="text-2xl">Server Splitter</CardTitle>
            <CardDescription>Este servidor ya es un subservidor. Solo los servidores principales pueden dividirse en subservidores.</CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  return (
    <div className="mt-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
      <Card className="border-0 bg-transparent shadow-none">
        <CardHeader className="px-0 flex flex-row items-center justify-between">
          <div>
            <CardTitle className="text-2xl">Server Splitter</CardTitle>
            <CardDescription>Distribuye los recursos totales de este servidor para crear múltiples subservidores.</CardDescription>
          </div>
          <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
            <DialogTrigger asChild>
              <Button>
                <Plus className="w-4 h-4 mr-2" />
                Crear Subservidor
              </Button>
            </DialogTrigger>
            <DialogContent className="max-w-5xl max-h-[88vh] flex flex-col">
              <CreateServerStepper 
                forcedParentId={serverId} 
                forcedNodeId={String(currentServerInfo?.nodeId || '')}
                onComplete={() => {
                  setIsDialogOpen(false);
                  refresh();
                }} 
              />
            </DialogContent>
          </Dialog>
        </CardHeader>
        <CardContent className="space-y-8 px-0 mt-4">
          
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <div className="border rounded-xl p-5 bg-accent/5 relative overflow-hidden">
              <div className="absolute top-0 right-0 p-4 opacity-5">
                <Cpu className="w-24 h-24" />
              </div>
              <div className="flex items-center gap-2 mb-2 text-muted-foreground relative z-10">
                <Cpu className="w-5 h-5" />
                <span className="text-sm font-semibold uppercase tracking-wide">CPU Total Asignada</span>
              </div>
              <div className="text-4xl font-black relative z-10">{currentServerInfo?.totalCpu || 0}%</div>
              <div className="text-sm text-muted-foreground mt-2 relative z-10 font-medium">
                Usado: <span className="text-primary">{usedCpuBySubservers}%</span> &nbsp;|&nbsp; Libre: <span className="text-green-500">{(currentServerInfo?.totalCpu || 0) - usedCpuBySubservers}%</span>
              </div>
            </div>
            
            <div className="border rounded-xl p-5 bg-accent/5 relative overflow-hidden">
              <div className="absolute top-0 right-0 p-4 opacity-5">
                <MemoryStick className="w-24 h-24" />
              </div>
              <div className="flex items-center gap-2 mb-2 text-muted-foreground relative z-10">
                <MemoryStick className="w-5 h-5" />
                <span className="text-sm font-semibold uppercase tracking-wide">Memoria RAM Total</span>
              </div>
              <div className="text-4xl font-black relative z-10">{currentServerInfo?.totalMemory || 0} MB</div>
              <div className="text-sm text-muted-foreground mt-2 relative z-10 font-medium">
                Usado: <span className="text-primary">{usedMemoryBySubservers} MB</span> &nbsp;|&nbsp; Libre: <span className="text-green-500">{(currentServerInfo?.totalMemory || 0) - usedMemoryBySubservers} MB</span>
              </div>
            </div>
            
            <div className="border rounded-xl p-5 bg-accent/5 relative overflow-hidden">
              <div className="absolute top-0 right-0 p-4 opacity-5">
                <HardDrive className="w-24 h-24" />
              </div>
              <div className="flex items-center gap-2 mb-2 text-muted-foreground relative z-10">
                <HardDrive className="w-5 h-5" />
                <span className="text-sm font-semibold uppercase tracking-wide">Disco Total</span>
              </div>
              <div className="text-4xl font-black relative z-10">{currentServerInfo?.totalDisk || 0} MB</div>
              <div className="text-sm text-muted-foreground mt-2 relative z-10 font-medium">
                Usado: <span className="text-primary">{usedDiskBySubservers} MB</span> &nbsp;|&nbsp; Libre: <span className="text-green-500">{(currentServerInfo?.totalDisk || 0) - usedDiskBySubservers} MB</span>
              </div>
            </div>
          </div>

          <div className="space-y-4 pt-4">
            <div className="flex flex-col gap-1">
              <div className="flex items-center gap-2">
                <div className="h-2 w-2 rounded-full bg-primary" />
                <h3 className="text-xl font-bold">Subservidores Activos</h3>
              </div>
              <Separator className="mt-2" />
            </div>

            {subServers.length === 0 ? (
              <div className="border-2 border-dashed rounded-xl p-12 text-center text-muted-foreground">
                No hay subservidores creados a partir de este servidor todavía.
              </div>
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 ml-4">
                {subServers.map(sub => (
                  <div key={sub.id} className="flex flex-col border rounded-xl p-5 bg-background shadow-sm hover:shadow-md transition-shadow">
                    <div className="flex items-center gap-3 mb-4">
                      <div className="bg-primary/10 p-2 rounded-lg">
                        <ServerIcon className="w-6 h-6 text-primary" />
                      </div>
                      <div>
                        <div className="font-bold text-lg leading-tight">{sub.name}</div>
                        <div className="text-xs text-muted-foreground font-mono">{sub.id}</div>
                      </div>
                    </div>
                    <div className="grid grid-cols-3 gap-2 text-sm text-muted-foreground bg-accent/5 p-3 rounded-lg mt-auto">
                      <div className="flex flex-col items-center">
                        <span className="text-xs uppercase tracking-wider font-semibold opacity-70">CPU</span>
                        <span className="font-bold text-foreground">{sub.totalCpu}%</span>
                      </div>
                      <div className="flex flex-col items-center border-l border-r border-border">
                        <span className="text-xs uppercase tracking-wider font-semibold opacity-70">RAM</span>
                        <span className="font-bold text-foreground">{sub.totalMemory}MB</span>
                      </div>
                      <div className="flex flex-col items-center">
                        <span className="text-xs uppercase tracking-wider font-semibold opacity-70">Disco</span>
                        <span className="font-bold text-foreground">{sub.totalDisk}MB</span>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
