'use client';
import { useAuth } from '@/contexts/providers';
import { useEffect, useState } from 'react';
import { PageHeader } from '@/components/page-header';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Code, Puzzle, Bot, Server as ServerIcon, Database, PlusCircle } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { useTranslations } from '@/contexts/translations-context';

type Template = {
  id: string;
  name: string;
  category: 'Web' | 'Game' | 'Bot' | 'Other';
  icon: LucideIcon;
};

// Mock data for templates
const templates: Template[] = [
  { id: 'tpl-1', name: 'Node.js', category: 'Web', icon: Code },
  { id: 'tpl-2', name: 'Minecraft (Forge)', category: 'Game', icon: Puzzle },
  { id: 'tpl-3', name: 'Discord Bot (Python)', category: 'Bot', icon: Bot },
  { id: 'tpl-4', name: 'Blank Server', category: 'Other', icon: ServerIcon },
  { id: 'tpl-5', name: 'React App', category: 'Web', icon: Code },
  { id: 'tpl-6', name: 'CS:GO Server', category: 'Game', icon: Puzzle },
  { id: 'tpl-7', name: 'Telegram Bot', category: 'Bot', icon: Bot },
  { id: 'tpl-8', name: 'PostgreSQL', category: 'Other', icon: Database },
];

const categoryColors: Record<Template['category'], string> = {
  Web: 'border-blue-500/30 bg-blue-500/10 text-blue-400',
  Game: 'border-green-500/30 bg-green-500/10 text-green-400',
  Bot: 'border-purple-500/30 bg-purple-500/10 text-purple-400',
  Other: 'border-slate-500/30 bg-slate-500/10 text-slate-400',
};


export default function TemplatesPage() {
  const { role } = useAuth();
  const [isMounted, setIsMounted] = useState(false);
  const { t } = useTranslations();

  useEffect(() => {
    setIsMounted(true);
    if (role && role !== 'admin') {
      window.location.href = '/dashboard';
    }
  }, [role]);

  if (!isMounted || role !== 'admin') {
    return (
      <div className="flex h-full items-center justify-center">
        <p>{t('common.loading')}</p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-8">
      <PageHeader title={t('templates.title')} description={t('templates.description')}>
        <Button>
          <PlusCircle className="mr-2" />
          {t('templates.create')}
        </Button>
      </PageHeader>

      <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        {templates.map((template) => (
          <div key={template.id} className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50 transition-all duration-300 hover:scale-105 hover:shadow-xl hover:shadow-primary/10">
            <Card className="flex h-full cursor-pointer flex-col justify-between overflow-hidden border-0">
              <CardHeader>
                <div className="flex items-start justify-between">
                  <template.icon className="h-10 w-10 text-muted-foreground" />
                  <Badge variant="outline" className={categoryColors[template.category]}>
                    {t(`templates.categories.${template.category.toLowerCase()}`)}
                  </Badge>
                </div>
              </CardHeader>
              <CardContent>
                <CardTitle className="text-lg">{template.name}</CardTitle>
              </CardContent>
              <div className="p-6 pt-2">
                <Button className="w-full">
                  {t('templates.use')}
                </Button>
              </div>
            </Card>
          </div>
        ))}
      </div>
    </div>
  );
}
