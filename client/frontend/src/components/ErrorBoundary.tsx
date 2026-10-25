'use client';
import React, { Component, type ErrorInfo, type ReactNode } from 'react';
import { Button } from '@/components/ui/button';
import { AlertCircle, RefreshCcw } from 'lucide-react';
import { useTranslations } from '@/contexts/translations-context';

interface Props {
    children?: ReactNode;
    fallback?: ReactNode;
    name?: string;
}

interface State {
    hasError: boolean;
    error: Error | null;
}

/** The default fallback is a function component so it can read the translations. */
function ErrorFallback({ message, onReset }: Readonly<{ message?: string; onReset: () => void }>) {
    const { t } = useTranslations();
    return (
        <div className="flex flex-col items-center justify-center p-8 text-center bg-destructive/5 rounded-lg border border-destructive/20 gap-4 mt-6">
            <div className="flex items-center gap-2 text-destructive">
                <AlertCircle className="h-6 w-6" />
                <h2 className="text-lg font-bold">{t('common.errorBoundary.title')}</h2>
            </div>
            <p className="text-sm text-muted-foreground max-w-md">
                {message || t('common.errorBoundary.description')}
            </p>
            <Button
                variant="outline"
                size="sm"
                onClick={onReset}
                className="mt-2"
            >
                <RefreshCcw className="mr-2 h-4 w-4" />
                {t('common.errorBoundary.tryAgain')}
            </Button>
        </div>
    );
}

class ErrorBoundary extends Component<Props, State> {
    public state: State = {
        hasError: false,
        error: null,
    };

    public static getDerivedStateFromError(error: Error): State {
        return { hasError: true, error };
    }

    public componentDidCatch(error: Error, errorInfo: ErrorInfo) {
        console.error(`Uncaught error in ${this.props.name || 'Component'}:`, error, errorInfo);
    }

    public render() {
        if (this.state.hasError) {
            if (this.props.fallback) {
                return this.props.fallback;
            }

            return <ErrorFallback message={this.state.error?.message} onReset={() => this.setState({ hasError: false, error: null })} />;
        }

        return this.props.children;
    }
}

export default ErrorBoundary;
