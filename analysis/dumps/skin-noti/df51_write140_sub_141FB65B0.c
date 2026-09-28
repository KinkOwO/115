// write140_sub_141FB65B0

__int64 __fastcall sub_141FB65B0(__int64 a1, unsigned __int8 *a2)
{
  int v4; // ebx
  __int64 v5; // rcx
  __int64 v6; // rax
  int i; // ebx
  __int64 v8; // rcx
  __int64 v9; // rax
  __int64 v10; // rcx
  __int64 v11; // rax
  __int64 v12; // rbp
  __int64 v13; // rcx
  __int64 v14; // rax
  unsigned int *j; // rbx
  __int64 v16; // rcx
  __int64 v17; // rax
  __int64 v18; // rax
  __int64 v19; // r8
  __int64 result; // rax
  __int64 v21; // [rsp+20h] [rbp-28h]
  __int64 v22; // [rsp+28h] [rbp-20h]
  __int64 v23; // [rsp+30h] [rbp-18h]

  v23 = -2;
  v4 = *((_DWORD *)a2 + 3) + *((_DWORD *)a2 + 4) + *((_DWORD *)a2 + 5) + *((_DWORD *)a2 + 6) + *((_DWORD *)a2 + 7);
  v5 = qword_14E650BF8;
  if ( !qword_14E650BF8 )
  {
    v6 = sub_146E8BA20(344);
    v22 = v6;
    if ( v6 )
      v6 = sub_1478692E0(v6);
    qword_14E650BF8 = v6;
    (**(void (__fastcall ***)(__int64))(v6 + 120))(v6 + 120);
    v5 = qword_14E650BF8;
  }
  if ( v4 >= (int)sub_14069DE00(v5) )
    *(_BYTE *)(a1 + 140) = 1;
  *(_DWORD *)(a1 + 124) = *a2;
  *(_DWORD *)(a1 + 128) = a2[4];
  *(_DWORD *)(a1 + 132) = a2[1];
  *(_DWORD *)(a1 + 136) = a2[5];
  for ( i = 0; ; ++i )
  {
    v8 = qword_14E650BF8;
    if ( !qword_14E650BF8 )
    {
      v9 = sub_146E8BA20(344);
      v22 = v9;
      if ( v9 )
        v9 = sub_1478692E0(v9);
      qword_14E650BF8 = v9;
      (**(void (__fastcall ***)(__int64))(v9 + 120))(v9 + 120);
      v8 = qword_14E650BF8;
    }
    if ( i >= (int)sub_1406FE620(v8) )
      break;
    v10 = qword_14E650BF8;
    if ( !qword_14E650BF8 )
    {
      v11 = sub_146E8BA20(344);
      v21 = v11;
      if ( v11 )
        v11 = sub_1478692E0(v11);
      qword_14E650BF8 = v11;
      (**(void (__fastcall ***)(__int64))(v11 + 120))(v11 + 120);
      v10 = qword_14E650BF8;
    }
    if ( *((_DWORD *)a2 + 12) <= (int)sub_147891850(v10, (unsigned int)i) )
      break;
  }
  if ( *(_DWORD *)(a1 + 144) != i )
  {
    if ( i > *((_DWORD *)a2 + 2) - 1 )
      i = *((_DWORD *)a2 + 2);
    *(_DWORD *)(a1 + 144) = i;
    if ( qword_14E66C090 )
    {
      v12 = sub_141308BD0(qword_14E66C090);
      if ( v12 )
      {
        v13 = qword_14E650BF8;
        if ( !qword_14E650BF8 )
        {
          v14 = sub_146E8BA20(344);
          v21 = v14;
          if ( v14 )
            v14 = sub_1478692E0(v14);
          qword_14E650BF8 = v14;
          (**(void (__fastcall ***)(__int64))(v14 + 120))(v14 + 120);
          v13 = qword_14E650BF8;
        }
        for ( j = *(unsigned int **)sub_141820810(v13); ; j += 3 )
        {
          v16 = qword_14E650BF8;
          if ( !qword_14E650BF8 )
          {
            v17 = sub_146E8BA20(344);
            v21 = v17;
            if ( v17 )
              v17 = sub_1478692E0(v17);
            qword_14E650BF8 = v17;
            (**(void (__fastcall ***)(__int64))(v17 + 120))(v17 + 120);
            v16 = qword_14E650BF8;
          }
          if ( j == *(unsigned int **)(sub_141820810(v16) + 8) )
            break;
          v18 = sub_146D03760(v12, *j, j[1], j[2], v21, v22, v23);
          if ( v18 )
          {
            LOBYTE(v19) = 1;
            sub_145B77D90(v18, *(unsigned int *)(a1 + 144), v19);
          }
        }
      }
    }
  }
  *(_OWORD *)(a1 + 72) = *(_OWORD *)a2;
  *(_OWORD *)(a1 + 88) = *((_OWORD *)a2 + 1);
  *(_OWORD *)(a1 + 104) = *((_OWORD *)a2 + 2);
  result = *((unsigned int *)a2 + 12);
  *(_DWORD *)(a1 + 120) = result;
  return result;
}

