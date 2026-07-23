import { describe, it, expect, beforeEach } from 'vitest';
import { useChatStore } from './chat';
import type { Conversation, Message } from './chat';

describe('useChatStore', () => {
  beforeEach(() => {
    useChatStore.setState({
      conversations: [],
      currentConversation: null,
      messages: [],
    });
  });

  it('initializes with empty state', () => {
    const state = useChatStore.getState();
    expect(state.conversations).toEqual([]);
    expect(state.currentConversation).toBeNull();
    expect(state.messages).toEqual([]);
  });

  it('setConversations updates conversations', () => {
    const convs: Conversation[] = [
      { id: '1', title: 'Test', lastMessageAt: Date.now() },
    ];
    useChatStore.getState().setConversations(convs);
    expect(useChatStore.getState().conversations).toEqual(convs);
  });

  it('setCurrentConversation updates current conversation', () => {
    useChatStore.getState().setCurrentConversation('123');
    expect(useChatStore.getState().currentConversation).toBe('123');
    useChatStore.getState().setCurrentConversation(null);
    expect(useChatStore.getState().currentConversation).toBeNull();
  });

  it('addMessage appends message to messages array', () => {
    const msg1: Message = { id: '1', content: 'Hello', role: 'user', timestamp: Date.now() };
    const msg2: Message = { id: '2', content: 'Hi', role: 'assistant', timestamp: Date.now() };

    useChatStore.getState().addMessage(msg1);
    expect(useChatStore.getState().messages).toHaveLength(1);
    expect(useChatStore.getState().messages[0]).toEqual(msg1);

    useChatStore.getState().addMessage(msg2);
    expect(useChatStore.getState().messages).toHaveLength(2);
    expect(useChatStore.getState().messages[1]).toEqual(msg2);
  });

  it('clearMessages empties messages array', () => {
    const msg: Message = { id: '1', content: 'Test', role: 'user', timestamp: Date.now() };
    useChatStore.getState().addMessage(msg);
    expect(useChatStore.getState().messages).toHaveLength(1);

    useChatStore.getState().clearMessages();
    expect(useChatStore.getState().messages).toEqual([]);
  });
});
