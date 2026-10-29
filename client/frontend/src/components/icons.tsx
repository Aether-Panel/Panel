// The panel's icons, drawn with Phosphor. Pages import them from here under the names they
// have always used, so the whole set is chosen in this one file.
import { forwardRef } from 'react';
import type { Icon, IconProps } from '@phosphor-icons/react';
import {
    ArchiveIcon,
    ArrowCircleUpIcon,
    ArrowClockwiseIcon,
    ArrowCounterClockwiseIcon,
    ArrowDownIcon,
    ArrowElbowDownLeftIcon,
    ArrowFatUpIcon,
    ArrowLeftIcon,
    ArrowLineUpIcon,
    ArrowRightIcon,
    ArrowSquareOutIcon,
    ArrowUpIcon,
    ArrowUpRightIcon,
    ArrowsClockwiseIcon,
    ArrowsCounterClockwiseIcon,
    ArrowsInIcon,
    ArrowsLeftRightIcon,
    ArrowsOutIcon,
    BellIcon,
    BellRingingIcon,
    BookOpenTextIcon,
    BracketsCurlyIcon,
    BroadcastIcon,
    CalendarCheckIcon,
    CaretDownIcon,
    CaretLeftIcon,
    CaretRightIcon,
    CaretUpIcon,
    CaretUpDownIcon,
    CellSignalHighIcon,
    CheckIcon,
    CheckCircleIcon,
    CircleIcon,
    ClockIcon,
    ClockCounterClockwiseIcon,
    CloudIcon,
    CloudArrowDownIcon,
    CloudArrowUpIcon,
    CodeIcon,
    CoffeeIcon,
    CopyIcon,
    CopySimpleIcon,
    CpuIcon,
    CrownIcon,
    CubeIcon,
    DatabaseIcon,
    DotsThreeIcon,
    DownloadSimpleIcon,
    EnvelopeIcon,
    EnvelopeSimpleOpenIcon,
    EraserIcon,
    EyeIcon,
    EyeSlashIcon,
    FadersIcon,
    FileIcon,
    FileAudioIcon,
    FileCodeIcon,
    FileDashedIcon,
    FileImageIcon,
    FilePlusIcon,
    FileTextIcon,
    FileVideoIcon,
    FileXlsIcon,
    FileZipIcon,
    FlagIcon,
    FloppyDiskIcon,
    FolderOpenIcon,
    FolderSimpleIcon,
    FolderSimplePlusIcon,
    GameControllerIcon,
    GaugeIcon,
    GearSixIcon,
    GitBranchIcon,
    GitDiffIcon,
    GlobeIcon,
    GlobeSimpleIcon,
    HardDriveIcon,
    HardDrivesIcon,
    HourglassIcon,
    HouseIcon,
    InfoIcon,
    KeyIcon,
    LayoutIcon,
    LightbulbIcon,
    ListIcon,
    ListChecksIcon,
    LockIcon,
    MagnifyingGlassIcon,
    MagnifyingGlassMinusIcon,
    MagnifyingGlassPlusIcon,
    MapPinIcon,
    MapTrifoldIcon,
    MemoryIcon,
    MinusIcon,
    MonitorPlayIcon,
    PackageIcon,
    PaletteIcon,
    PaperPlaneRightIcon,
    PencilLineIcon,
    PencilSimpleIcon,
    PlayIcon,
    PlugsIcon,
    PlusIcon,
    PowerIcon,
    ProhibitIcon,
    PulseIcon,
    PuzzlePieceIcon,
    RobotIcon,
    RocketIcon,
    ScissorsIcon,
    ShieldIcon,
    ShieldCheckIcon,
    ShieldWarningIcon,
    SidebarSimpleIcon,
    SignInIcon,
    SignOutIcon,
    SlidersHorizontalIcon,
    SparkleIcon,
    SpinnerGapIcon,
    SquaresFourIcon,
    StackIcon,
    StarIcon,
    StopIcon,
    TerminalWindowIcon,
    TrashIcon,
    TreeStructureIcon,
    UploadSimpleIcon,
    UserIcon,
    UserPlusIcon,
    UsersIcon,
    WarningIcon,
    WarningCircleIcon,
    WarningOctagonIcon,
    WebhooksLogoIcon,
    WrenchIcon,
    XIcon,
    XCircleIcon,
} from '@phosphor-icons/react';

export type AppIcon = Icon;
export type { IconProps, IconWeight } from '@phosphor-icons/react';

// 24px unless a size is given (a size class such as h-4 w-4 still wins), and hidden from screen
// readers unless it is labelled
function panelIcon(Base: Icon): Icon {
    const PanelIcon = forwardRef<SVGSVGElement, IconProps>((props, ref) => (
        <Base ref={ref} size={24} aria-hidden={props['aria-label'] || props.alt ? undefined : true} {...props} />
    ));
    PanelIcon.displayName = Base.displayName;
    return PanelIcon;
}

export const Activity = panelIcon(PulseIcon);
export const AlertCircle = panelIcon(WarningCircleIcon);
export const AlertOctagon = panelIcon(WarningOctagonIcon);
export const AlertTriangle = panelIcon(WarningIcon);
export const Archive = panelIcon(ArchiveIcon);
export const ArrowBigUp = panelIcon(ArrowFatUpIcon);
export const ArrowDown = panelIcon(ArrowDownIcon);
export const ArrowLeft = panelIcon(ArrowLeftIcon);
export const ArrowRight = panelIcon(ArrowRightIcon);
export const ArrowRightLeft = panelIcon(ArrowsLeftRightIcon);
export const ArrowUp = panelIcon(ArrowUpIcon);
export const ArrowUpCircle = panelIcon(ArrowCircleUpIcon);
export const ArrowUpRight = panelIcon(ArrowUpRightIcon);
export const ArrowUpToLine = panelIcon(ArrowLineUpIcon);
export const Ban = panelIcon(ProhibitIcon);
export const Bell = panelIcon(BellIcon);
export const BellRing = panelIcon(BellRingingIcon);
export const Blocks = panelIcon(CubeIcon);
export const BookOpenText = panelIcon(BookOpenTextIcon);
export const Bot = panelIcon(RobotIcon);
export const Box = panelIcon(CubeIcon);
export const CalendarClock = panelIcon(CalendarCheckIcon);
export const Check = panelIcon(CheckIcon);
export const CheckCircle2 = panelIcon(CheckCircleIcon);
export const ChevronDown = panelIcon(CaretDownIcon);
export const ChevronLeft = panelIcon(CaretLeftIcon);
export const ChevronRight = panelIcon(CaretRightIcon);
export const ChevronUp = panelIcon(CaretUpIcon);
export const ChevronsUpDown = panelIcon(CaretUpDownIcon);
export const Circle = panelIcon(CircleIcon);
export const CircleCheck = panelIcon(CheckCircleIcon);
export const CircleX = panelIcon(XCircleIcon);
export const Clock = panelIcon(ClockIcon);
export const Cloud = panelIcon(CloudIcon);
export const CloudUpload = panelIcon(CloudArrowUpIcon);
export const Code = panelIcon(CodeIcon);
export const Coffee = panelIcon(CoffeeIcon);
export const Copy = panelIcon(CopyIcon);
export const CopyPlus = panelIcon(CopySimpleIcon);
export const CornerDownLeft = panelIcon(ArrowElbowDownLeftIcon);
export const Cpu = panelIcon(CpuIcon);
export const Crown = panelIcon(CrownIcon);
export const Database = panelIcon(DatabaseIcon);
export const Download = panelIcon(DownloadSimpleIcon);
export const DownloadCloud = panelIcon(CloudArrowDownIcon);
export const Edit2 = panelIcon(PencilSimpleIcon);
export const Eraser = panelIcon(EraserIcon);
export const ExternalLink = panelIcon(ArrowSquareOutIcon);
export const Eye = panelIcon(EyeIcon);
export const EyeOff = panelIcon(EyeSlashIcon);
export const File = panelIcon(FileIcon);
export const FileArchive = panelIcon(FileZipIcon);
export const FileAudio = panelIcon(FileAudioIcon);
export const FileCode = panelIcon(FileCodeIcon);
export const FileCog = panelIcon(FileCodeIcon);
export const FileImage = panelIcon(FileImageIcon);
export const FileJson = panelIcon(BracketsCurlyIcon);
export const FilePlus = panelIcon(FilePlusIcon);
export const FileQuestion = panelIcon(FileDashedIcon);
export const FileSpreadsheet = panelIcon(FileXlsIcon);
export const FileTerminal = panelIcon(FileCodeIcon);
export const FileText = panelIcon(FileTextIcon);
export const FileVideo = panelIcon(FileVideoIcon);
export const Flag = panelIcon(FlagIcon);
export const Folder = panelIcon(FolderSimpleIcon);
export const FolderOpen = panelIcon(FolderOpenIcon);
export const FolderPlus = panelIcon(FolderSimplePlusIcon);
export const Gamepad2 = panelIcon(GameControllerIcon);
export const Gauge = panelIcon(GaugeIcon);
export const GitBranch = panelIcon(GitBranchIcon);
export const GitCompareArrows = panelIcon(GitDiffIcon);
export const Globe = panelIcon(GlobeSimpleIcon);
export const Globe2 = panelIcon(GlobeIcon);
export const HardDrive = panelIcon(HardDriveIcon);
export const History = panelIcon(ClockCounterClockwiseIcon);
export const Home = panelIcon(HouseIcon);
export const Hourglass = panelIcon(HourglassIcon);
export const Info = panelIcon(InfoIcon);
export const Key = panelIcon(KeyIcon);
export const KeyRound = panelIcon(KeyIcon);
export const Layers = panelIcon(StackIcon);
export const LayoutDashboard = panelIcon(SquaresFourIcon);
export const LayoutTemplate = panelIcon(LayoutIcon);
export const Lightbulb = panelIcon(LightbulbIcon);
export const ListChecks = panelIcon(ListChecksIcon);
export const Loader2 = panelIcon(SpinnerGapIcon);
export const Lock = panelIcon(LockIcon);
export const LogIn = panelIcon(SignInIcon);
export const LogOut = panelIcon(SignOutIcon);
export const Mail = panelIcon(EnvelopeIcon);
export const MailCheck = panelIcon(EnvelopeSimpleOpenIcon);
const MapIcon = panelIcon(MapTrifoldIcon);
export { MapIcon as Map };
export const MapPin = panelIcon(MapPinIcon);
export const Maximize2 = panelIcon(ArrowsOutIcon);
export const MemoryStick = panelIcon(MemoryIcon);
export const Menu = panelIcon(ListIcon);
export const Minimize2 = panelIcon(ArrowsInIcon);
export const Minus = panelIcon(MinusIcon);
export const MonitorPlay = panelIcon(MonitorPlayIcon);
export const MoreHorizontal = panelIcon(DotsThreeIcon);
export const Network = panelIcon(TreeStructureIcon);
export const Package = panelIcon(PackageIcon);
export const Palette = panelIcon(PaletteIcon);
export const PanelLeft = panelIcon(SidebarSimpleIcon);
export const Pencil = panelIcon(PencilSimpleIcon);
export const PencilLine = panelIcon(PencilLineIcon);
export const Play = panelIcon(PlayIcon);
export const PlugZap = panelIcon(PlugsIcon);
export const Plus = panelIcon(PlusIcon);
export const Power = panelIcon(PowerIcon);
export const Puzzle = panelIcon(PuzzlePieceIcon);
export const Radio = panelIcon(BroadcastIcon);
export const RefreshCcw = panelIcon(ArrowsCounterClockwiseIcon);
export const RefreshCw = panelIcon(ArrowsClockwiseIcon);
export const Rocket = panelIcon(RocketIcon);
export const RotateCcw = panelIcon(ArrowCounterClockwiseIcon);
export const RotateCw = panelIcon(ArrowClockwiseIcon);
export const Save = panelIcon(FloppyDiskIcon);
export const Scissors = panelIcon(ScissorsIcon);
export const ScrollText = panelIcon(ClockCounterClockwiseIcon);
export const Search = panelIcon(MagnifyingGlassIcon);
export const SearchCheck = panelIcon(MagnifyingGlassPlusIcon);
export const SearchX = panelIcon(MagnifyingGlassMinusIcon);
export const Send = panelIcon(PaperPlaneRightIcon);
export const Server = panelIcon(HardDrivesIcon);
export const ServerIcon = panelIcon(HardDrivesIcon);
export const ServerOff = panelIcon(HardDrivesIcon);
export const Settings = panelIcon(GearSixIcon);
export const Settings2 = panelIcon(FadersIcon);
export const Shield = panelIcon(ShieldIcon);
export const ShieldAlert = panelIcon(ShieldWarningIcon);
export const ShieldCheck = panelIcon(ShieldCheckIcon);
export const SignalHigh = panelIcon(CellSignalHighIcon);
export const SlidersHorizontal = panelIcon(SlidersHorizontalIcon);
export const Sparkles = panelIcon(SparkleIcon);
export const Square = panelIcon(StopIcon);
export const SquareTerminal = panelIcon(TerminalWindowIcon);
export const Star = panelIcon(StarIcon);
export const Terminal = panelIcon(TerminalWindowIcon);
export const Trash2 = panelIcon(TrashIcon);
export const TriangleAlert = panelIcon(WarningIcon);
export const Upload = panelIcon(UploadSimpleIcon);
export const UploadCloud = panelIcon(CloudArrowUpIcon);
export const User = panelIcon(UserIcon);
export const UserPlus = panelIcon(UserPlusIcon);
export const Users = panelIcon(UsersIcon);
export const Webhook = panelIcon(WebhooksLogoIcon);
export const Wrench = panelIcon(WrenchIcon);
export const X = panelIcon(XIcon);
export const XCircle = panelIcon(XCircleIcon);
