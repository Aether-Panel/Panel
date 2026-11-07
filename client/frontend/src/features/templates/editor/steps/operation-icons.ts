import {
  Archive, ArrowRightLeft, Blocks, Coffee, CornerDownLeft, Download, DownloadCloud, FileArchive, FileCode, FilePlus, FileQuestion, FolderPlus, Gamepad2,
  Github, Hourglass, Info, Layers, Package, PencilLine, Play, Puzzle, RotateCw, SearchCheck, Square, Terminal, Wrench, type AppIcon,
} from '@/components/icons';
import type { OperationDefinition, OperationIconName } from '../../schema/operations';

// The icons the registry names (operations.ts), imported one by one: looking them up in the
// whole icon module by name would put every icon of the panel in the editor's bundle. A test
// checks every operation's icon is here.

export const OPERATION_ICONS: Partial<Record<OperationIconName, AppIcon>> = {
  Archive, ArrowRightLeft, Blocks, Coffee, CornerDownLeft, Download, DownloadCloud, FileArchive, FileCode, FilePlus, FolderPlus, Gamepad2,
  Github, Hourglass, Info, Layers, Package, PencilLine, Play, Puzzle, RotateCw, SearchCheck, Square, Terminal, Wrench,
};

/** The icon of an operation; a step of a type the panel does not know gets a question mark. */
export function operationIcon(operation: OperationDefinition | undefined): AppIcon {
  return (operation && OPERATION_ICONS[operation.icon]) ?? FileQuestion;
}
