import { describe, expect, it } from 'vitest'
import { FilterBuilder } from '../src/filters'

/** Create a fresh FilterBuilder and return it with its params for inspection. */
function createFilter(): { builder: FilterBuilder<unknown>; params: URLSearchParams } {
  const params = new URLSearchParams()
  const headers: Record<string, string> = {}
  const builder = new FilterBuilder(params, headers)
  return { builder, params }
}

describe('FilterBuilder', () => {
  it('eq() produces column=eq.value', () => {
    const { builder, params } = createFilter()
    builder.eq('status', 'active')
    expect(params.get('status')).toBe('eq.active')
  })

  it('neq() produces column=neq.value', () => {
    const { builder, params } = createFilter()
    builder.neq('role', 'admin')
    expect(params.get('role')).toBe('neq.admin')
  })

  it('gt() produces column=gt.value', () => {
    const { builder, params } = createFilter()
    builder.gt('age', '18')
    expect(params.get('age')).toBe('gt.18')
  })

  it('gte() produces column=gte.value', () => {
    const { builder, params } = createFilter()
    builder.gte('score', '100')
    expect(params.get('score')).toBe('gte.100')
  })

  it('lt() produces column=lt.value', () => {
    const { builder, params } = createFilter()
    builder.lt('price', '50')
    expect(params.get('price')).toBe('lt.50')
  })

  it('lte() produces column=lte.value', () => {
    const { builder, params } = createFilter()
    builder.lte('count', '10')
    expect(params.get('count')).toBe('lte.10')
  })

  it('like() produces column=like.pattern', () => {
    const { builder, params } = createFilter()
    builder.like('name', '%john%')
    expect(params.get('name')).toBe('like.%john%')
  })

  it('ilike() produces column=ilike.pattern', () => {
    const { builder, params } = createFilter()
    builder.ilike('email', '%@EXAMPLE.COM')
    expect(params.get('email')).toBe('ilike.%@EXAMPLE.COM')
  })

  it('is() produces column=is.value for null/true/false', () => {
    const { builder, params } = createFilter()
    builder.is('deleted_at', 'null')
    expect(params.get('deleted_at')).toBe('is.null')
  })

  it('in() produces column=in.(v1,v2,v3)', () => {
    const { builder, params } = createFilter()
    builder.in('status', ['active', 'pending', 'review'])
    expect(params.get('status')).toBe('in.(active,pending,review)')
  })

  it('contains() produces column=cs.value', () => {
    const { builder, params } = createFilter()
    builder.contains('tags', '{postgres,sql}')
    expect(params.get('tags')).toBe('cs.{postgres,sql}')
  })

  it('containedBy() produces column=cd.value', () => {
    const { builder, params } = createFilter()
    builder.containedBy('tags', '{a,b,c}')
    expect(params.get('tags')).toBe('cd.{a,b,c}')
  })

  it('not() produces column=not.op.value', () => {
    const { builder, params } = createFilter()
    builder.not('status', 'eq', 'deleted')
    expect(params.get('status')).toBe('not.eq.deleted')
  })

  it('or() produces or=(filter1,filter2)', () => {
    const { builder, params } = createFilter()
    builder.or('status.eq.active,status.eq.pending')
    expect(params.get('or')).toBe('(status.eq.active,status.eq.pending)')
  })

  it('textSearch() produces column=fts.query by default', () => {
    const { builder, params } = createFilter()
    builder.textSearch('body', 'hello world')
    expect(params.get('body')).toBe('fts.hello world')
  })

  it('textSearch() with plain type produces column=plfts.query', () => {
    const { builder, params } = createFilter()
    builder.textSearch('body', 'hello', { type: 'plain' })
    expect(params.get('body')).toBe('plfts.hello')
  })

  it('textSearch() with phrase type produces column=phfts.query', () => {
    const { builder, params } = createFilter()
    builder.textSearch('body', 'hello world', { type: 'phrase' })
    expect(params.get('body')).toBe('phfts.hello world')
  })

  it('textSearch() with web type produces column=wfts.query', () => {
    const { builder, params } = createFilter()
    builder.textSearch('body', 'hello', { type: 'web' })
    expect(params.get('body')).toBe('wfts.hello')
  })

  it('textSearch() with config produces column=fts(config).query', () => {
    const { builder, params } = createFilter()
    builder.textSearch('body', 'hello', { config: 'english' })
    expect(params.get('body')).toBe('fts(english).hello')
  })

  it('chaining multiple filters accumulates params', () => {
    const { builder, params } = createFilter()
    builder
      .eq('active', 'true')
      .gt('age', '18')
      .lt('age', '65')

    expect(params.get('active')).toBe('eq.true')
    // gt and lt on the same column use append, so getAll returns both
    const ageParams = params.getAll('age')
    expect(ageParams).toContain('gt.18')
    expect(ageParams).toContain('lt.65')
  })

  it('each filter method returns the same builder instance', () => {
    const { builder } = createFilter()
    const result = builder.eq('a', '1').neq('b', '2').gt('c', '3')
    expect(result).toBe(builder)
  })
})
