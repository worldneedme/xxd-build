import 'dart:async';

import 'package:fl_clash/common/common.dart';
import 'package:fl_clash/common/permission.dart';
import 'package:fl_clash/common/system_dns.dart';
import 'package:fl_clash/enum/enum.dart';
import 'package:fl_clash/manager/window_manager.dart';
import 'package:fl_clash/models/models.dart';
import 'package:fl_clash/providers/providers.dart';
import 'package:fl_clash/state.dart';
import 'package:fl_clash/widgets/animated_visibility.dart';
import 'package:flutter/foundation.dart';
import 'package:material_ui/material_ui.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

class AppStateManager extends ConsumerStatefulWidget {
  final Widget child;

  const AppStateManager({super.key, required this.child});

  @override
  ConsumerState<AppStateManager> createState() => _AppStateManagerState();
}

class _AppStateManagerState extends ConsumerState<AppStateManager>
    with WidgetsBindingObserver {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    ref.listenManual(checkIpProvider, (prev, next) {
      if (prev != next && next.isInit && next.containsDetection) {
        ref.read(networkDetectionProvider.notifier).startCheck();
      }
    });
    ref.listenManual(configProvider, (prev, next) {
      if (prev != next) {
        ref.read(storeActionProvider.notifier).savePreferencesDebounce();
      }
    });
    ref.listenManual(needUpdateGroupsProvider, (prev, next) {
      if (prev != next) {
        ref.read(proxiesActionProvider.notifier).updateGroupsDebounce();
      }
    });
    ref.listenManual(suspendProvider, (prev, next) {
      final isStart = ref.read(isStartProvider);
      if (prev != next && isStart) {
        debouncer.call(FunctionTag.suspend, () async {
          final core = ref.read(coreHandlerProvider);
          if (next == true) {
            await core.stopListener();
          } else {
            await core.startListener();
          }
          ref.read(checkIpNumProvider.notifier).add();
        });
      }
    });
    final systemDns = systemDnsCoordinator;
    if (systemDns != null) {
      ref.listenManual(shouldPatchSystemDnsProvider, (prev, next) {
        unawaited(systemDns.sync(next));
      }, fireImmediately: true);
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  Future<void> didChangeAppLifecycleState(AppLifecycleState state) async {
    commonPrint.log('$state');
    if (state == AppLifecycleState.resumed) {
      permissions.check(ref.read);
      render?.resume();
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) {
          return;
        }
        ref.read(setupActionProvider.notifier).tryCheckIp();
      });
    }
  }

  @override
  void didChangePlatformBrightness() {
    ref.read(themeActionProvider.notifier).updateBrightness();
  }

  @override
  Widget build(BuildContext context) {
    return Listener(
      onPointerHover: (_) {
        render?.resume();
      },
      child: widget.child,
    );
  }
}

class AppEnvManager extends StatelessWidget {
  final Widget child;

  const AppEnvManager({super.key, required this.child});

  @override
  Widget build(BuildContext context) {
    if (kDebugMode) {
      if (globalState.isPre) {
        return Banner(
          message: 'DEBUG',
          location: BannerLocation.topEnd,
          child: child,
        );
      }
    }
    if (globalState.isPre) {
      return Banner(
        message: globalState.appEnv.toUpperCase(),
        location: BannerLocation.topEnd,
        child: child,
      );
    }
    return child;
  }
}

int _sidebarGroupOf(PageLabel label) => switch (label) {
  PageLabel.dashboard || PageLabel.proxies || PageLabel.profiles => 0,
  PageLabel.tools => 2,
  _ => 1,
};

class SidebarNav extends StatelessWidget {
  const SidebarNav({
    super.key,
    required this.items,
    required this.selectedIndex,
    required this.extended,
    required this.onSelected,
  });

  final List<NavigationItem> items;
  final int selectedIndex;
  final bool extended;
  final void Function(int index) onSelected;

  @override
  Widget build(BuildContext context) {
    return FocusTraversalGroup(
      child: ListView(
        padding: EdgeInsets.symmetric(horizontal: extended ? 12 : 10),
        children: [
          for (var i = 0; i < items.length; i++) ...[
            if (i > 0 &&
                _sidebarGroupOf(items[i].label) !=
                    _sidebarGroupOf(items[i - 1].label))
              const SizedBox(height: 14),
            _SidebarItem(
              item: items[i],
              selected: i == selectedIndex,
              extended: extended,
              onTap: () => onSelected(i),
            ),
          ],
        ],
      ),
    );
  }
}

class _SidebarItem extends StatelessWidget {
  const _SidebarItem({
    required this.item,
    required this.selected,
    required this.extended,
    required this.onTap,
  });

  final NavigationItem item;
  final bool selected;
  final bool extended;
  final VoidCallback onTap;

  static final _radius = AppRadius.all(10);

  @override
  Widget build(BuildContext context) {
    final colorScheme = context.colorScheme;
    final label = item.label.label;
    final foreground = selected
        ? colorScheme.onSurface
        : colorScheme.onSurfaceVariant;
    // The tree shape must not depend on [selected]: re-parenting the InkWell
    // would drop its focus node right after keyboard activation.
    final Widget row = SizedBox(
      height: 40,
      child: Row(
        mainAxisAlignment: extended
            ? MainAxisAlignment.start
            : MainAxisAlignment.center,
        children: [
          if (extended) const SizedBox(width: 12),
          IconTheme.merge(
            data: IconThemeData(
              size: 20,
              color: selected ? colorScheme.primary : foreground,
            ),
            child: item.icon,
          ),
          if (extended) ...[
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                label,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: context.textTheme.labelLarge?.copyWith(
                  color: foreground,
                  fontWeight: selected ? FontWeight.w600 : FontWeight.w500,
                ),
              ),
            ),
          ],
        ],
      ),
    );
    final Widget body = Semantics(
      selected: selected,
      child: Stack(
        children: [
          Positioned.fill(
            child: AnimatedOpacity(
              opacity: selected ? 1 : 0,
              duration: const Duration(milliseconds: 180),
              child: _SidebarPill(radius: _radius),
            ),
          ),
          Material(
            type: MaterialType.transparency,
            child: InkWell(
              onTap: onTap,
              customBorder: RoundedSuperellipseBorder(borderRadius: _radius),
              child: row,
            ),
          ),
        ],
      ),
    );
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 1),
      child: extended ? body : Tooltip(message: label, child: body),
    );
  }
}

class _SidebarPill extends StatelessWidget {
  const _SidebarPill({required this.radius});

  final BorderRadius radius;

  @override
  Widget build(BuildContext context) {
    final isDark = context.colorScheme.brightness == Brightness.dark;
    return DecoratedBox(
      decoration: ShapeDecoration(
        shape: RoundedSuperellipseBorder(
          borderRadius: radius,
          side: BorderSide(
            color: isDark
                ? Colors.white.withValues(alpha: 0.10)
                : Colors.black.withValues(alpha: 0.06),
            width: hairline,
          ),
        ),
        gradient: LinearGradient(
          begin: Alignment.topCenter,
          end: Alignment.bottomCenter,
          colors: isDark
              ? [
                  Colors.white.withValues(alpha: 0.13),
                  Colors.white.withValues(alpha: 0.07),
                ]
              : [
                  Colors.white.withValues(alpha: 0.96),
                  Colors.white.withValues(alpha: 0.78),
                ],
        ),
        shadows: [
          BoxShadow(
            color: Colors.black.withValues(alpha: isDark ? 0.30 : 0.06),
            blurRadius: 6,
            offset: const Offset(0, 1),
          ),
        ],
      ),
    );
  }
}

class _SidebarBrand extends StatelessWidget {
  const _SidebarBrand({required this.extended});

  final bool extended;

  @override
  Widget build(BuildContext context) {
    final icon = ClipRSuperellipse(
      borderRadius: AppRadius.all(8),
      child: Image.asset(
        'assets/images/icon.png',
        width: 30,
        height: 30,
        fit: BoxFit.cover,
      ),
    );
    if (!extended) {
      return icon;
    }
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 22),
      child: Row(
        children: [
          icon,
          const SizedBox(width: 10),
          Expanded(
            child: Text(
              brandName,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: context.textTheme.titleSmall?.copyWith(
                fontWeight: FontWeight.w700,
                letterSpacing: 0.6,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _ContentSheet extends StatelessWidget {
  const _ContentSheet({required this.framed, required this.child});

  final bool framed;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final colorScheme = context.colorScheme;
    final isDark = colorScheme.brightness == Brightness.dark;
    // Same widget shape framed or not, so switching layouts keeps page state.
    final radius = AppRadius.all(framed ? 12.0 : 0.0);
    return DecoratedBox(
      position: DecorationPosition.foreground,
      decoration: ShapeDecoration(
        shape: RoundedSuperellipseBorder(
          borderRadius: radius,
          side: framed
              ? BorderSide(color: colorScheme.outlineVariant, width: hairline)
              : BorderSide.none,
        ),
      ),
      child: DecoratedBox(
        decoration: ShapeDecoration(
          color: colorScheme.surface,
          shape: RoundedSuperellipseBorder(borderRadius: radius),
          shadows: [
            if (framed)
              BoxShadow(
                color: Colors.black.withValues(alpha: isDark ? 0.30 : 0.05),
                blurRadius: 4,
                offset: const Offset(0, 1),
              ),
          ],
        ),
        child: ClipRSuperellipse(borderRadius: radius, child: child),
      ),
    );
  }
}

class AppSidebarContainer extends ConsumerWidget {
  final Widget child;

  const AppSidebarContainer({super.key, required this.child});

  static const _extendedMinViewWidth = 720.0;

  void _updateSideBarWidth(WidgetRef ref, double contentWidth) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(sideWidthProvider.notifier).value =
          ref.read(viewSizeProvider.select((state) => state.width)) -
          contentWidth;
    });
  }

  void _handleToPage(WidgetRef ref, PageLabel pageLabel) {
    final focusNode = FocusManager.instance.primaryFocus;
    final preserveNavigationFocus =
        focusNode?.context?.findAncestorWidgetOfExactType<SidebarNav>() != null;
    ref.read(currentPageLabelProvider.notifier).toPage(pageLabel);
    if (!preserveNavigationFocus || focusNode == null) {
      return;
    }
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (focusNode.context != null && focusNode.canRequestFocus) {
        focusNode.requestFocus();
      }
    });
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final navigationState = ref.watch(navigationStateProvider);
    final navigationItems = navigationState.navigationItems;
    final isMobileView = navigationState.viewMode == ViewMode.mobile;
    final currentIndex = navigationState.currentIndex;
    final extended =
        ref.watch(viewSizeProvider.select((state) => state.width)) >=
        _extendedMinViewWidth;
    // The desktop root paints the aurora; sidebar and title bar sit straight
    // on it so they read as one frosted strip around the content sheet.
    return Row(
      children: [
        AnimatedVisibility.sidebar(
          visible: !isMobileView,
          child: SizedBox(
            width: extended ? 220 : 76,
            child: Material(
              type: MaterialType.transparency,
              child: SafeArea(
                right: false,
                child: Column(
                  children: [
                    if (system.isMacOS) const SizedBox(height: 22),
                    const SizedBox(height: 10),
                    if (!system.isMacOS) ...[
                      _SidebarBrand(extended: extended),
                      const SizedBox(height: 18),
                    ],
                    Expanded(
                      child: ScrollConfiguration(
                        behavior: const HiddenBarScrollBehavior(),
                        child: SidebarNav(
                          items: navigationItems,
                          selectedIndex: currentIndex,
                          extended: extended,
                          onSelected: (index) {
                            _handleToPage(ref, navigationItems[index].label);
                          },
                        ),
                      ),
                    ),
                    const SizedBox(height: 12),
                  ],
                ),
              ),
            ),
          ),
        ),
        Expanded(
          flex: 1,
          child: LayoutBuilder(
            builder: (_, constraints) {
              _updateSideBarWidth(ref, constraints.maxWidth);
              return Padding(
                padding: isMobileView
                    ? EdgeInsets.zero
                    : const EdgeInsets.fromLTRB(0, 0, 8, 8),
                child: _ContentSheet(framed: !isMobileView, child: child),
              );
            },
          ),
        ),
      ],
    );
  }
}
