// sender_1551_sub_14276C420

void __fastcall sub_14276C420(__int64 a1, __int64 a2, __int64 a3)
{
  unsigned int v4; // eax
  __int64 v5; // rdx
  __int64 v6; // r8
  __int64 v7; // r9
  __int64 v8; // rax
  int v9; // eax
  __int64 v10; // rcx
  __int64 v11; // rax
  __int64 v12; // rcx
  __int64 v13; // rax
  __int64 v14; // rdx
  __int64 v15; // rdx
  __int64 v16; // rcx
  _QWORD *v17; // rdx
  volatile signed __int32 *v18; // rbx
  unsigned __int64 v19; // rdx
  __int64 v20; // rcx
  __int64 v21; // [rsp+20h] [rbp-48h] BYREF
  volatile signed __int32 *v22; // [rsp+28h] [rbp-40h]
  __int64 v23; // [rsp+30h] [rbp-38h]
  _QWORD v24[2]; // [rsp+38h] [rbp-30h] BYREF
  __int64 v25; // [rsp+48h] [rbp-20h]
  unsigned __int64 v26; // [rsp+50h] [rbp-18h]

  v23 = -2;
  if ( qword_14E682918 )
  {
    v4 = sub_146E9F840(a1 + 344);
    sub_145E295B0(qword_14E682918, v4);
    sub_146E9FF70(a1 + 344, v5, v6, v7);
  }
  if ( qword_14E683C78 && sub_1429BDDE0(qword_14E683C78) )
  {
    v8 = sub_1429BDDE0(qword_14E683C78);
    sub_1455262F0(v8);
  }
  v9 = *(_DWORD *)(a1 + 448);
  if ( v9 == 1 )
  {
    if ( !*(_BYTE *)(a1 + 688) )
    {
      sub_14276E890(a1, a1 + 824, 1);
      *(_BYTE *)(a1 + 688) = 1;
      *(_BYTE *)(a1 + 1196) = 1;
    }
    LOBYTE(a3) = 1;
    sub_1421BA850(a1, a1 + 696, a3);
  }
  else if ( v9 == 2 )
  {
    v24[0] = 0;
    v25 = 0;
    v26 = 7;
    sub_14014C8D0(v24, &byte_14BAF7F08);
    if ( v25 )
    {
      if ( qword_14E683C78 )
      {
        v17 = v24;
        if ( v26 >= 8 )
          v17 = (_QWORD *)v24[0];
        sub_1466775B0(qword_14E683C78, v17, 1);
        sub_14667F660(qword_14E683C78, &v21);
        if ( v21 && !(unsigned __int8)sub_144AD3560() )
          sub_144AD3CD0(v21);
        v18 = v22;
        if ( v22 )
        {
          if ( _InterlockedExchangeAdd(v22 + 2, 0xFFFFFFFF) == 1 )
          {
            (**(void (__fastcall ***)(volatile signed __int32 *))v18)(v18);
            if ( _InterlockedExchangeAdd(v18 + 3, 0xFFFFFFFF) == 1 )
              (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v18 + 8LL))(v18);
          }
        }
      }
    }
    else
    {
      v11 = sub_146D74000(v10);
      sub_146D746E0(v11, 1646);
      v13 = sub_146D74000(v12);
      LOBYTE(v14) = 1;
      sub_146D75CC0(v13, v14);
      sub_146D75AF0(v16, v15);
    }
    if ( v26 >= 8 )
    {
      v19 = 2 * v26 + 2;
      v20 = v24[0];
      if ( v19 >= 0x1000 )
      {
        v19 = 2 * v26 + 41;
        v20 = *(_QWORD *)(v24[0] - 8LL);
        if ( (unsigned __int64)(v24[0] - v20 - 8) > 0x1F )
          sub_148AAF304(v20, v19);
      }
      sub_146E9F3A0(v20, v19);
    }
    v25 = 0;
    v26 = 7;
    LOWORD(v24[0]) = 0;
  }
}

