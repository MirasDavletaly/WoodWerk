/* ==========================================================================
   WOODWERK — переключение языка сайта
   Русский, казахский, английский.

   Русский — исходный язык: он уже в разметке, поэтому словарь для него
   не грузится. Перед первой подменой запоминаем исходный текст каждого
   размеченного элемента, и возврат на русский — это просто восстановление.
   ========================================================================== */
(function () {
  'use strict';

  var LANGS = [
    { code: 'ru', short: 'Рус', label: 'Русский', htmlLang: 'ru' },
    { code: 'kk', short: 'Қаз', label: 'Қазақша', htmlLang: 'kk' },
    { code: 'en', short: 'Eng', label: 'English', htmlLang: 'en' }
  ];
  var DEFAULT = 'ru';
  var STORAGE_KEY = 'ww_lang';

  // Адрес папки со словарями считаем от самого скрипта: страница может
  // открываться и как /catalog, и как /product/12 — относительный путь
  // в этих случаях указывал бы в разные места.
  var BASE = (function () {
    var src = (document.currentScript && document.currentScript.src) || '';
    var cut = src.indexOf('/assets/js/');
    return cut > -1 ? src.slice(0, cut) : '';
  })();

  var dictionaries = {};   // код языка -> словарь
  var original = null;     // исходный русский текст, снимается один раз
  var currentLang = DEFAULT;

  /* ------------------------------------------------------------------
     Выбор языка
     ------------------------------------------------------------------ */

  function known(code) {
    for (var i = 0; i < LANGS.length; i++) {
      if (LANGS[i].code === code) return LANGS[i];
    }
    return null;
  }

  function stored() {
    try {
      return window.localStorage.getItem(STORAGE_KEY);
    } catch (err) {
      return null;   // приватный режим — просто без запоминания
    }
  }

  function remember(code) {
    try {
      window.localStorage.setItem(STORAGE_KEY, code);
    } catch (err) { /* не страшно */ }
  }

  // Порядок: адрес страницы, прошлый выбор, язык браузера, русский.
  function pickInitial() {
    var fromURL = new URLSearchParams(window.location.search).get('lang');
    if (known(fromURL)) return fromURL;

    var saved = stored();
    if (known(saved)) return saved;

    var browser = (navigator.language || '').slice(0, 2).toLowerCase();
    if (known(browser)) return browser;

    return DEFAULT;
  }

  /* ------------------------------------------------------------------
     Подмена текста
     ------------------------------------------------------------------ */

  var ATTR_MAP = {
    'data-i18n-placeholder': 'placeholder',
    'data-i18n-title': 'title',
    'data-i18n-alt': 'alt',
    'data-i18n-aria-label': 'aria-label',
    'data-i18n-content': 'content'
  };

  function snapshot() {
    if (original) return;
    original = { html: {}, attrs: [] };

    var nodes = document.querySelectorAll('[data-i18n]');
    for (var i = 0; i < nodes.length; i++) {
      original.html[nodes[i].getAttribute('data-i18n')] = nodes[i].innerHTML;
    }

    for (var dataAttr in ATTR_MAP) {
      if (!Object.prototype.hasOwnProperty.call(ATTR_MAP, dataAttr)) continue;
      var withAttr = document.querySelectorAll('[' + dataAttr + ']');
      for (var j = 0; j < withAttr.length; j++) {
        original.attrs.push({
          node: withAttr[j],
          attr: ATTR_MAP[dataAttr],
          value: withAttr[j].getAttribute(ATTR_MAP[dataAttr]) || ''
        });
      }
    }
  }

  function apply(dict) {
    snapshot();

    var nodes = document.querySelectorAll('[data-i18n]');
    for (var i = 0; i < nodes.length; i++) {
      var key = nodes[i].getAttribute('data-i18n');
      var value = dict ? dict[key] : null;
      // Текст в словаре — наш собственный файл, не пользовательский ввод.
      nodes[i].innerHTML = (value !== undefined && value !== null)
        ? value
        : original.html[key];
    }

    for (var k = 0; k < original.attrs.length; k++) {
      var item = original.attrs[k];
      var dataName = 'data-i18n-' + item.attr;
      var attrKey = item.node.getAttribute(dataName);
      var translated = dict && attrKey ? dict[attrKey] : null;
      item.node.setAttribute(item.attr,
        (translated !== undefined && translated !== null) ? translated : item.value);
    }
  }

  /* ------------------------------------------------------------------
     Загрузка словаря
     ------------------------------------------------------------------ */

  function loadDictionary(code) {
    if (code === DEFAULT) return Promise.resolve(null);
    if (dictionaries[code]) return Promise.resolve(dictionaries[code]);

    return fetch(BASE + '/assets/i18n/' + code + '.json', { credentials: 'same-origin' })
      .then(function (res) {
        if (!res.ok) throw new Error('HTTP ' + res.status);
        return res.json();
      })
      .then(function (dict) {
        dictionaries[code] = dict;
        return dict;
      });
  }

  function setLanguage(code, save) {
    var lang = known(code) ? code : DEFAULT;

    return loadDictionary(lang).then(function (dict) {
      apply(dict);
      currentLang = lang;
      document.documentElement.setAttribute('lang', known(lang).htmlLang);
      markSwitcher(lang);
      if (save) remember(lang);
      document.dispatchEvent(new CustomEvent('ww:lang', { detail: { lang: lang } }));
    }).catch(function () {
      // Словарь не загрузился — остаёмся на русском, сайт продолжает работать.
      apply(null);
      currentLang = DEFAULT;
      markSwitcher(DEFAULT);
    });
  }

  /* ------------------------------------------------------------------
     Кнопка выбора языка
     ------------------------------------------------------------------ */

  function markSwitcher(code) {
    var buttons = document.querySelectorAll('[data-lang]');
    for (var i = 0; i < buttons.length; i++) {
      var on = buttons[i].getAttribute('data-lang') === code;
      buttons[i].classList.toggle('is-active', on);
      buttons[i].setAttribute('aria-pressed', String(on));
    }
  }

  function bindSwitcher() {
    document.addEventListener('click', function (e) {
      var button = e.target.closest ? e.target.closest('[data-lang]') : null;
      if (!button) return;
      e.preventDefault();
      var code = button.getAttribute('data-lang');
      if (code === currentLang) return;
      setLanguage(code, true);
    });
  }

  /* ------------------------------------------------------------------
     Запуск
     ------------------------------------------------------------------ */

  function start() {
    bindSwitcher();
    var initial = pickInitial();
    if (initial === DEFAULT) {
      markSwitcher(DEFAULT);
      return;   // страница уже на русском, подменять нечего
    }
    setLanguage(initial, false);
  }

  // Язык нужен и другим скриптам — например каталогу, который просит
  // у сервера названия изделий на нужном языке.
  window.WWLang = {
    current: function () { return currentLang; },
    set: function (code) { return setLanguage(code, true); },
    list: LANGS
  };

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', start);
  } else {
    start();
  }
})();
