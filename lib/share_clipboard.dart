import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import 'core_bridge.dart';
import 'local_store.dart';
import 'models.dart';

bool looksLikeShareClipboard(String text) {
  final value = text.trim();
  if (value.isEmpty || value.length > 4096) return false;
  return value.contains('http') ||
      value.contains('《') ||
      value.contains('红果') ||
      value.contains('番茄');
}

Future<bool?> showShareConfirmDialog(BuildContext context, String title) {
  final hasTitle = title.trim().isNotEmpty && title.trim() != '短剧';
  return showDialog<bool>(
    context: context,
    builder: (context) => AlertDialog(
      title: const Text('发现短剧口令'),
      content: Text(
        hasTitle ? '检测到这部短剧的分享口令，是否立即打开播放？\n\n$title' : '检测到一个短剧分享链接，是否立即打开播放？',
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context, false),
          child: const Text('取消'),
        ),
        FilledButton(
          onPressed: () => Navigator.pop(context, true),
          child: const Text('立即播放'),
        ),
      ],
    ),
  );
}

class ShareClipboardWatcher {
  ShareClipboardWatcher({
    required this.repository,
    required this.store,
    required this.onOpen,
  });

  final AppRepository repository;
  final LocalStore store;
  final void Function(Drama drama) onOpen;

  String? _openedText;
  bool _handling = false;

  void dispose() {
    unawaited(repository.cancelShare());
  }

  Future<void> check(BuildContext context) async {
    if (_handling ||
        !context.mounted ||
        store.locked ||
        !store.allowsSource('hongguo')) {
      return;
    }
    final route = ModalRoute.of(context);
    if (route != null && !route.isCurrent) return;
    String text;
    try {
      final data = await Clipboard.getData(Clipboard.kTextPlain);
      text = data?.text?.trim() ?? '';
    } catch (_) {
      return;
    }
    if (!looksLikeShareClipboard(text) || text == _openedText) return;
    _handling = true;
    try {
      final drama = await repository.resolveShare(text);
      if (drama == null || !context.mounted) return;
      final confirmed = await showShareConfirmDialog(context, drama.title);
      if (confirmed != true || !context.mounted) return;
      _openedText = text;
      onOpen(drama);
    } catch (_) {
    } finally {
      _handling = false;
    }
  }
}
