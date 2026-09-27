// slotreader_sub_1448D1560_0x1448d1560

void __fastcall sub_1448D1560(_QWORD *a1, char a2, int a3)
{
  __int64 *v5; // r9
  __int64 *v6; // rcx
  __int64 *v7; // rax
  unsigned __int64 v8; // rsi
  int v9; // eax
  __int64 *v10; // r9
  int v11; // ebp
  __int64 *v12; // rdx
  __int64 *v13; // rcx
  _DWORD *v14; // rax
  unsigned int v15; // ebx
  __int64 v16; // rax
  __int64 v17; // rdx
  unsigned __int64 v18; // rcx
  __int64 v19; // rax
  __int64 v20; // rcx
  __int64 v21; // rax
  __int64 v22; // rsi
  int v23; // eax
  int v24; // r9d
  __int64 v25; // rcx
  __int64 v26; // rdx
  int v27; // ecx
  int v28; // eax
  __int128 v29; // [rsp+40h] [rbp-28h]
  int v30; // [rsp+80h] [rbp+18h] BYREF

  v30 = a3;
  if ( *((_BYTE *)a1 + 936) )
  {
    v5 = (__int64 *)a1[115];
    v6 = v5;
    v7 = (__int64 *)v5[1];
    while ( !*((_BYTE *)v7 + 25) )
    {
      if ( *((_DWORD *)v7 + 7) >= a3 )
      {
        v6 = v7;
        v7 = (__int64 *)*v7;
      }
      else
      {
        v7 = (__int64 *)v7[2];
      }
    }
    if ( !*((_BYTE *)v6 + 25) && a3 >= *((_DWORD *)v6 + 7) && v6 != v5 )
    {
      v8 = (*(int (__fastcall **)(_QWORD *, _QWORD))(*a1 + 1632LL))(a1, 0);
      v9 = (*(__int64 (__fastcall **)(_QWORD *, __int64))(*a1 + 1632LL))(a1, 4);
      v10 = (__int64 *)a1[136];
      v11 = v9;
      v12 = v10;
      v13 = (__int64 *)v10[1];
      while ( !*((_BYTE *)v13 + 25) )
      {
        if ( *((_DWORD *)v13 + 7) >= v30 )
        {
          v12 = v13;
          v13 = (__int64 *)*v13;
        }
        else
        {
          v13 = (__int64 *)v13[2];
        }
      }
      if ( *((_BYTE *)v12 + 25) || v30 < *((_DWORD *)v12 + 7) || v12 == v10 )
        *(_DWORD *)sub_1401C4620(a1 + 136, &v30) = 0;
      if ( a1[133] < v8 && *(_DWORD *)sub_1401C4620(a1 + 136, &v30) < v11 )
      {
        v14 = (_DWORD *)sub_1401C4620(a1 + 136, &v30);
        BYTE4(v29) = 0;
        DWORD2(v29) = 0;
        ++*v14;
        LODWORD(v29) = v30;
        if ( a2 == 1 )
        {
          BYTE4(v29) = 1;
          DWORD2(v29) = sub_146E9F840(a1 + 125);
        }
        v15 = sub_1446B9B50();
        if ( v15 == 3 )
        {
          HIDWORD(v29) = 0;
        }
        else
        {
          v16 = (*(__int64 (__fastcall **)(_QWORD *))(*a1 + 1416LL))(a1);
          HIDWORD(v29) = sub_1446B9600(v15, v16);
        }
        v17 = a1[133];
        v18 = a1[131];
        if ( v18 <= v17 + 1 )
        {
          sub_140BCFA40(a1 + 129, 1);
          v18 = a1[131];
          v17 = a1[133];
        }
        v19 = a1[132] & (v18 - 1);
        a1[132] = v19;
        v20 = (v17 + v19) & (v18 - 1);
        v21 = a1[130];
        v22 = 8 * v20;
        if ( !*(_QWORD *)(8 * v20 + v21) )
        {
          *(_QWORD *)(v22 + a1[130]) = sub_146E8BA20(16);
          v21 = a1[130];
        }
        *(_OWORD *)*(_QWORD *)(v22 + v21) = v29;
        ++a1[133];
        v23 = sub_146E8C7D0(&unk_14A446DE0);
        v25 = a1[20];
        if ( v25 && *(_DWORD *)(v25 + 8) )
          v26 = a1[21];
        else
          v26 = 0;
        v27 = v26 - 48;
        if ( !v26 )
          v27 = 0;
        LOBYTE(v24) = 1;
        sub_1456C6EA0(v27, v23, 2, v24, 1, 0, 0);
        v28 = sub_146E8C7D0(&unk_14A446E60);
        sub_14538A020((_DWORD)a1, v28, -1, 0, 0, 0, 0, 1);
      }
    }
  }
}

