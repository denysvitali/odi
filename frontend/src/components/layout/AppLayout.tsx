import { Suspense, useEffect, useState } from 'react'
import {
  ActionIcon,
  AppShell,
  Badge,
  Burger,
  Center,
  Group,
  Kbd,
  Loader,
  NavLink,
  ScrollArea,
  Stack,
  Text,
  ThemeIcon,
  Tooltip,
  UnstyledButton,
  useComputedColorScheme,
  useMantineColorScheme
} from '@mantine/core'
import { useDisclosure, useHotkeys, useNetwork } from '@mantine/hooks'
import { spotlight } from '@mantine/spotlight'
import {
  IconFileSearch,
  IconFiles,
  IconKeyboard,
  IconLink,
  IconMessageCircle,
  IconMoon,
  IconSearch,
  IconSettings,
  IconStar,
  IconSun,
  IconUpload,
  IconWifiOff
} from '@tabler/icons-react'
import { Link, NavLink as RouterNavLink, Outlet, useLocation, useNavigate } from 'react-router'
import { favorites } from '@/stores/local'
import { DocumentDrawer } from '@/components/documents/DocumentDrawer'
import { CommandPalette } from './CommandPalette'
import { DropOverlay } from './DropOverlay'
import { ShortcutsModal } from './ShortcutsModal'
import { useGoShortcuts } from './useGoShortcuts'

const NAV = [
  { to: '/', label: 'Search', icon: IconSearch, end: true },
  { to: '/documents', label: 'All documents', icon: IconFiles },
  { to: '/favorites', label: 'Favorites', icon: IconStar },
  { to: '/upload', label: 'Upload', icon: IconUpload },
  { to: '/chat', label: 'Ask', icon: IconMessageCircle }
]
const NAV_SECONDARY = [
  { to: '/shares', label: 'Shares', icon: IconLink },
  { to: '/admin', label: 'Admin', icon: IconSettings }
]

export function AppLayout() {
  const [navOpen, nav] = useDisclosure(false)
  const [shortcutsOpen, shortcuts] = useDisclosure(false)
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const favoriteCount = favorites.use().length
  const { online } = useNetwork()

  useEffect(() => {
    nav.close()
    shortcuts.close()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pathname])

  useHotkeys([
    ['shift+?', shortcuts.open],
    [
      '/',
      () => {
        const input = document.querySelector<HTMLInputElement>('input[data-global-search]')
        if (input) input.focus()
        else navigate('/')
      }
    ]
  ])
  useGoShortcuts(navigate)

  const link = (item: (typeof NAV)[number]) => (
    <NavLink
      key={item.to}
      component={RouterNavLink}
      to={item.to}
      end={item.end}
      label={item.label}
      leftSection={<item.icon size={18} stroke={1.6} />}
      rightSection={
        item.to === '/favorites' && favoriteCount > 0 ? (
          <Badge size="xs" variant="light" circle>
            {favoriteCount}
          </Badge>
        ) : undefined
      }
    />
  )

  return (
    <AppShell
      header={{ height: 56 }}
      navbar={{ width: 220, breakpoint: 'sm', collapsed: { mobile: !navOpen } }}
      padding="md"
    >
      <AppShell.Header>
        <Group h="100%" px="md" gap="sm" wrap="nowrap">
          <Burger
            opened={navOpen}
            onClick={nav.toggle}
            hiddenFrom="sm"
            size="sm"
            aria-label={navOpen ? 'Close navigation' : 'Open navigation'}
          />
          <Link to="/" style={{ color: 'inherit', textDecoration: 'none' }} aria-label="ODI home">
            <Group gap={6}>
              <ThemeIcon size="md" radius="md" variant="filled">
                <IconFileSearch size={16} />
              </ThemeIcon>
              <Text fw={700}>ODI</Text>
            </Group>
          </Link>
          <SearchTrigger />
          {!online && (
            <Tooltip label="You are offline">
              <IconWifiOff size={18} color="var(--mantine-color-red-6)" aria-label="Offline" />
            </Tooltip>
          )}
        </Group>
      </AppShell.Header>

      <AppShell.Navbar p="xs" aria-label="Primary">
        <AppShell.Section grow component={ScrollArea}>
          <Stack gap={2}>{NAV.map(link)}</Stack>
        </AppShell.Section>
        <AppShell.Section>
          <Stack gap={2}>{NAV_SECONDARY.map(link)}</Stack>
          <Group gap="xs" mt="xs" px="xs">
            <ThemeToggle />
            <Tooltip label="Keyboard shortcuts (?)">
              <ActionIcon
                variant="subtle"
                color="gray"
                aria-label="Keyboard shortcuts"
                onClick={shortcuts.open}
              >
                <IconKeyboard size={18} />
              </ActionIcon>
            </Tooltip>
          </Group>
        </AppShell.Section>
      </AppShell.Navbar>

      <AppShell.Main id="main">
        <Suspense
          fallback={
            <Center py="xl">
              <Loader size="sm" />
            </Center>
          }
        >
          <Outlet />
        </Suspense>
      </AppShell.Main>

      <DocumentDrawer />
      <CommandPalette onShowShortcuts={shortcuts.open} />
      <ShortcutsModal opened={shortcutsOpen} onClose={shortcuts.close} />
      <DropOverlay />
    </AppShell>
  )
}

function SearchTrigger() {
  const [mac] = useState(() => /Mac|iPhone|iPad/.test(navigator.platform))
  return (
    <UnstyledButton
      onClick={spotlight.open}
      aria-label="Open command palette"
      ml="auto"
      px="sm"
      py={6}
      w={{ base: 'auto', sm: 260 }}
      style={{
        border: '1px solid var(--mantine-color-default-border)',
        borderRadius: 'var(--mantine-radius-md)'
      }}
    >
      <Group gap="xs" wrap="nowrap">
        <IconSearch size={16} color="var(--mantine-color-dimmed)" />
        <Text size="sm" c="dimmed" visibleFrom="sm" style={{ flex: 1 }}>
          Jump to…
        </Text>
        <Kbd size="xs" visibleFrom="sm">
          {mac ? '⌘' : 'Ctrl'} K
        </Kbd>
      </Group>
    </UnstyledButton>
  )
}

function ThemeToggle() {
  const { setColorScheme } = useMantineColorScheme()
  const dark = useComputedColorScheme('light') === 'dark'
  const label = dark ? 'Switch to light mode' : 'Switch to dark mode'
  return (
    <Tooltip label={label}>
      <ActionIcon
        variant="subtle"
        color="gray"
        aria-label={label}
        onClick={() => setColorScheme(dark ? 'light' : 'dark')}
      >
        {dark ? <IconSun size={18} /> : <IconMoon size={18} />}
      </ActionIcon>
    </Tooltip>
  )
}
